package main

import (
	"testing"
	"time"

	"github.com/keithy/sensible/pkg/sensible"
)

// MockStorage implements TaskRepository for testing
type MockStorage struct {
	tasks map[string]*sensible.Task
}

func NewMockStorage() *MockStorage {
	return &MockStorage{tasks: make(map[string]*sensible.Task)}
}

func (m *MockStorage) Save(task *sensible.Task) error {
	m.tasks[task.FileID] = task
	return nil
}

func (m *MockStorage) Load(id string) (*sensible.Task, error) {
	if t, ok := m.tasks[id]; ok {
		return t, nil
	}
	return nil, nil
}

func (m *MockStorage) ListPending() ([]*sensible.Task, error) {
	var result []*sensible.Task
	for _, t := range m.tasks {
		result = append(result, t)
	}
	return result, nil
}

func (m *MockStorage) MoveToDone(task *sensible.Task) error {
	task.Status = "done"
	m.tasks[task.FileID] = task
	return nil
}

func (m *MockStorage) Delete(id string) error {
	delete(m.tasks, id)
	return nil
}

func (m *MockStorage) AddTask(fileID, status, dependsOn string) *sensible.Task {
	task := &sensible.Task{
		FileID:    fileID,
		ID:        fileID,
		Request:   "test",
		Status:    status,
		Timestamp: time.Now().Format(time.RFC3339Nano),
		DependsOn: dependsOn,
	}
	m.tasks[fileID] = task
	return task
}

func TestFindReadyTask_ChainStopsOnFailure(t *testing.T) {
	storage := NewMockStorage()

	// Task1 is queued (will fail)
	storage.AddTask("task1", "queued", "")

	// Task2 depends on task1
	storage.AddTask("task2", "queued", "task1")

	// Task3 depends on task2
	storage.AddTask("task3", "queued", "task2")

	// First call: task1 is ready
	task, stopped := findReadyTask(storage)
	if task == nil && !stopped {
		t.Fatal("expected task1 to be ready or chain to stop")
	}
	if task != nil && task.FileID != "task1" {
		t.Errorf("expected task1, got %s", task.FileID)
	}
	if stopped {
		t.Error("should not be stopped yet")
	}

	// Simulate task1 failing - update in storage
	storage.tasks["task1"].Status = "failed"

	// Process task2 - it depends on failed task1
	task, stopped = findReadyTask(storage)
	// Chain should stop - task2 either failed or blocked
	if task != nil && !stopped {
		t.Error("expected either no task or chain stopped")
	}
}

func TestFindReadyTask_ChainContinuesOnSuccess(t *testing.T) {
	storage := NewMockStorage()

	// Task1 is queued
	storage.AddTask("task1", "queued", "")

	// Task2 depends on task1
	storage.AddTask("task2", "queued", "task1")

	// First call: task1 is ready
	task, _ := findReadyTask(storage)
	if task.FileID != "task1" {
		t.Errorf("expected task1, got %s", task.FileID)
	}

	// Simulate task1 succeeding
	storage.tasks["task1"].Status = "success"

	// Second call: task2 should now be ready
	task, stopped := findReadyTask(storage)
	if task == nil {
		t.Fatal("expected task2 to be ready after task1 succeeded")
	}
	if task.FileID != "task2" {
		t.Errorf("expected task2, got %s", task.FileID)
	}
	if stopped {
		t.Error("should not be stopped")
	}
}

func TestFindReadyTask_WaitsForParent(t *testing.T) {
	storage := NewMockStorage()

	// Task1 is running (not success/failed yet)
	storage.AddTask("task1", "running", "")

	// Task2 depends on task1
	storage.AddTask("task2", "queued", "task1")

	// Task2 should NOT be ready (parent still running)
	task, stopped := findReadyTask(storage)
	if task != nil {
		t.Error("expected no ready task (parent running)")
	}
	if stopped {
		t.Error("should not be stopped, just waiting")
	}
}

func TestFindReadyTask_NoDepsRunsImmediately(t *testing.T) {
	storage := NewMockStorage()

	// Task with no dependencies
	storage.AddTask("standalone", "queued", "")

	result, stopped := findReadyTask(storage)
	if result == nil {
		t.Fatal("expected task to be ready (no deps)")
	}
	if result.FileID != "standalone" {
		t.Errorf("expected standalone, got %s", result.FileID)
	}
	if stopped {
		t.Error("should not be stopped")
	}
}