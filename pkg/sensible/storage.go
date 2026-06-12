package sensible

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

// Storage implements TaskRepository using disk filesystem
type Storage struct {
	TasksDir string
}

// NewStorage creates a new Storage instance
func NewStorage(tasksDir string) *Storage {
	return &Storage{TasksDir: tasksDir}
}

// PendingDir returns the pending tasks directory path
func (s *Storage) PendingDir() string {
	return filepath.Join(s.TasksDir, "pending")
}

// Save saves a task to the pending directory
func (s *Storage) Save(task *Task) error {
	if err := os.MkdirAll(s.pendingDir(), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(s.pendingDir(), task.FileID+".json")
	return os.WriteFile(path, data, 0644)
}

// Load reads a task from disk by FileID
// Searches in: pending/, then done/
func (s *Storage) Load(id string) (*Task, error) {
	paths := []string{
		filepath.Join(s.pendingDir(), id+".json"),
		filepath.Join(s.doneDir(), id+".json"),
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err == nil {
			var task Task
			if err := json.Unmarshal(data, &task); err == nil {
				return &task, nil
			}
		}
	}
	return nil, nil
}

// ListPending returns all pending tasks sorted by modification time (oldest first)
func (s *Storage) ListPending() ([]*Task, error) {
	pattern := filepath.Join(s.pendingDir(), "*.json")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		return []*Task{}, nil
	}

	// Sort by modification time
	sort.Slice(files, func(i, j int) bool {
		iInfo, _ := os.Stat(files[i])
		jInfo, _ := os.Stat(files[j])
		if iInfo == nil || jInfo == nil {
			return false
		}
		return iInfo.ModTime().Before(jInfo.ModTime())
	})

	tasks := make([]*Task, 0, len(files))
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var task Task
		if err := json.Unmarshal(data, &task); err == nil {
			tasks = append(tasks, &task)
		}
	}
	return tasks, nil
}

// MoveToDone moves a completed task to the done directory
func (s *Storage) MoveToDone(task *Task) error {
	if err := os.MkdirAll(s.doneDir(), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(s.doneDir(), task.FileID+".json")
	return os.WriteFile(path, data, 0644)
}

// Delete removes a task from pending (after moving to done)
func (s *Storage) Delete(fileID string) error {
	path := filepath.Join(s.pendingDir(), fileID+".json")
	return os.Remove(path)
}

// pendingDir returns the pending tasks directory path
func (s *Storage) pendingDir() string {
	return filepath.Join(s.TasksDir, "pending")
}

// doneDir returns the done tasks directory path
func (s *Storage) doneDir() string {
	return filepath.Join(s.TasksDir, "done")
}

// ExeExecutor implements Executor by running tasks via execlineb
type ExeExecutor struct {}

// NewExeExecutor creates a new executor
func NewExeExecutor() *ExeExecutor {
	return &ExeExecutor{}
}