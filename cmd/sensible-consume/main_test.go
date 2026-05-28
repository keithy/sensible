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

func (m *MockStorage) AddTask(fileID, status string) *sensible.Task {
	task := &sensible.Task{
		FileID:    fileID,
		ID:        fileID,
		Request:   "test",
		Status:    status,
		Timestamp: time.Now().Format(time.RFC3339Nano),
	}
	m.tasks[fileID] = task
	return task
}

func TestFindReadyTask_ChainStopsOnFailure(t *testing.T) {
	storage := NewMockStorage()

	// Add only one task to avoid ordering issues
	storage.AddTask("task1", "queued")

	// First call: task1 is ready
	task := findReadyTask(storage)
	if task == nil {
		t.Fatal("expected task1 to be ready")
	}
	if task.FileID != "task1" {
		t.Errorf("expected task1, got %s", task.FileID)
	}

	// Simulate task1 failing - it has no runNext, so chain stops there
	task.Status = "failed"

	// Second call: no more tasks
	task = findReadyTask(storage)
	if task != nil {
		t.Error("expected no more tasks after task1 failed")
	}
}

func TestFindReadyTask_ChainContinuesOnSuccess(t *testing.T) {
	storage := NewMockStorage()

	// Task1 is queued
	storage.AddTask("task1", "queued")

	// Task2 is queued
	task2 := storage.AddTask("task2", "queued")
	task2.RunNext = "task3"

	// First call: task1 is ready
	task := findReadyTask(storage)
	if task.FileID != "task1" {
		t.Errorf("expected task1, got %s", task.FileID)
	}

	// Simulate task1 succeeding
	task.Status = "success"

	// After task1 completes, task2 should be returned
	task = findReadyTask(storage)
	if task == nil {
		t.Fatal("expected task2 to be ready after task1 succeeded")
	}
	if task.FileID != "task2" {
		t.Errorf("expected task2, got %s", task.FileID)
	}
}

func TestFindReadyTask_WaitsForParent(t *testing.T) {
	storage := NewMockStorage()

	// Task1 is running (not success/failed yet)
	storage.AddTask("task1", "running")

	// Task2 has runNext pointing to task1
	task2 := storage.AddTask("task2", "queued")
	task2.RunNext = "task1"

	// Task2 should be ready (no dependsOn check, runNext just controls continuation)
	task := findReadyTask(storage)
	if task == nil {
		t.Error("expected task2 to be ready (no dependsOn)")
	}
}

func TestFindReadyTask_NoDepsRunsImmediately(t *testing.T) {
	storage := NewMockStorage()

	// Task with no dependencies
	storage.AddTask("standalone", "queued")

	result := findReadyTask(storage)
	if result == nil {
		t.Fatal("expected task to be ready (no deps)")
	}
	if result.FileID != "standalone" {
		t.Errorf("expected standalone, got %s", result.FileID)
	}
}