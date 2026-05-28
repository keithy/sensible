package sensible

import (
	"fmt"
	"time"
)

// Task represents a stored task result
type Task struct {
	ID         string `json:"id"`          // Action name, e.g. "compile"
	FileID     string `json:"file_id"`     // Unique filename, e.g. "2026-04-30T12:00:00.123Z-compile"
	Request    string `json:"request,omitempty"`
	Status     string `json:"status"`
	RunNext    string `json:"run_next,omitempty"` // FileID of next task to run after this one
	DependsOn  string `json:"depends_on,omitempty"` // Deprecated: FileID of parent task
	ExitCode   int    `json:"exit_code,omitempty"`
	Stdout     string `json:"stdout,omitempty"`
	Stderr     string `json:"stderr,omitempty"`
	DurationMs int64  `json:"duration_ms,omitempty"`
	Timestamp  string `json:"timestamp"`   // RFC3339Nano
}

// Result holds execution result
type Result struct {
	Status     string
	ExitCode   int
	Stdout     string
	Stderr     string
	DurationMs int64
}

// TaskRepository defines storage operations
type TaskRepository interface {
	Save(task *Task) error
	Load(id string) (*Task, error)
	ListPending() ([]*Task, error)
	MoveToDone(task *Task) error
	Delete(fileID string) error
}

// Executor defines task execution
type Executor interface {
	Execute(req string, timeout int) *Result
}

// NewTask creates a new task with the given action and request
func NewTask(action, request string) *Task {
	timestamp := time.Now().UTC().Format(time.RFC3339Nano)
	return &Task{
		ID:        action,
		FileID:    fmt.Sprintf("%s-%s", timestamp, action),
		Request:   request,
		Status:    "queued",
		Timestamp: timestamp,
	}
}

// SetRunNext sets the next task in the chain
func (t *Task) SetRunNext(nextFileID string) {
	t.RunNext = nextFileID
}