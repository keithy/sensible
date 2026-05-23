package sensible

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewTask(t *testing.T) {
	task := NewTask("status", "status")
	if task.ID != "status" {
		t.Errorf("expected ID 'status', got %q", task.ID)
	}
	if task.FileID == "" {
		t.Error("FileID should not be empty")
	}
	if task.Status != "queued" {
		t.Errorf("expected status 'queued', got %q", task.Status)
	}
	if task.Request != "status" {
		t.Errorf("expected request 'status', got %q", task.Request)
	}
	if task.Timestamp == "" {
		t.Error("Timestamp should not be empty")
	}
}

func TestCreateDependentTask(t *testing.T) {
	task := CreateDependentTask("parent-123", "compile", "compile --target=linux")
	if task.DependsOn != "parent-123" {
		t.Errorf("expected DependsOn 'parent-123', got %q", task.DependsOn)
	}
	if task.ID != "compile" {
		t.Errorf("expected ID 'compile', got %q", task.ID)
	}
}

func TestStorage_SaveAndLoad(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sensible-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage := NewStorage(tmpDir)

	task := &Task{
		ID:        "test-action",
		FileID:    "2026-04-30T12:00:00Z-test-action",
		Status:    "success",
		ExitCode:  0,
		Stdout:    "test output",
		Timestamp: "2026-04-30T12:00:00Z",
	}

	if err := storage.Save(task); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	loaded, err := storage.Load(task.FileID)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if loaded == nil {
		t.Fatal("Load() returned nil")
	}
	if loaded.ID != task.ID {
		t.Errorf("ID mismatch: got %s, want %s", loaded.ID, task.ID)
	}
	if loaded.Stdout != task.Stdout {
		t.Errorf("Stdout mismatch: got %s, want %s", loaded.Stdout, task.Stdout)
	}
}

func TestStorage_MoveToDone(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sensible-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage := NewStorage(tmpDir)

	task := &Task{
		ID:        "test-action",
		FileID:    "2026-04-30T12:00:00Z-test-action",
		Status:    "success",
		Timestamp: "2026-04-30T12:00:00Z",
	}

	if err := storage.Save(task); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	if err := storage.MoveToDone(task); err != nil {
		t.Fatalf("MoveToDone() failed: %v", err)
	}

	// Should still exist in both (caller must Delete from pending after MoveToDone)
	donePath := filepath.Join(tmpDir, "done", task.FileID+".json")
	if _, err := os.Stat(donePath); os.IsNotExist(err) {
		t.Error("task should be in done directory")
	}
}

func TestStorage_ListPending(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sensible-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage := NewStorage(tmpDir)

	// Create some tasks
	for i := 0; i < 3; i++ {
		task := &Task{
			ID:        "action",
			FileID:    time.Now().UTC().Format(time.RFC3339Nano) + "-action",
			Status:    "queued",
			Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		}
		storage.Save(task)
		time.Sleep(1 * time.Millisecond) // ensure different timestamps
	}

	tasks, err := storage.ListPending()
	if err != nil {
		t.Fatalf("ListPending() failed: %v", err)
	}
	if len(tasks) != 3 {
		t.Errorf("expected 3 tasks, got %d", len(tasks))
	}
}

func TestStorage_Delete(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sensible-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage := NewStorage(tmpDir)

	task := &Task{
		ID:        "test-action",
		FileID:    "2026-04-30T12:00:00Z-test-action",
		Status:    "queued",
		Timestamp: "2026-04-30T12:00:00Z",
	}

	storage.Save(task)
	storage.Delete(task.FileID)

	loaded, _ := storage.Load(task.FileID)
	if loaded != nil {
		t.Error("task should be deleted")
	}
}

func TestStorage_Load_PendingThenDone(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sensible-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage := NewStorage(tmpDir)

	task := &Task{
		ID:        "test-action",
		FileID:    "2026-04-30T12:00:00Z-test-action",
		Status:    "success",
		Timestamp: "2026-04-30T12:00:00Z",
	}

	// Save to pending
	storage.Save(task)

	// Load should find it in pending
	loaded, _ := storage.Load(task.FileID)
	if loaded == nil {
		t.Fatal("should find task in pending")
	}

	// Move to done
	storage.MoveToDone(task)
	storage.Delete(task.FileID)

	// Load should now find it in done
	loaded, _ = storage.Load(task.FileID)
	if loaded == nil {
		t.Fatal("should find task in done")
	}
}

func TestGetActionTimeout(t *testing.T) {
	whitelist := []ActionConfig{
		{Name: "status", Timeout: 10},
		{Name: "compile", Timeout: 600},
	}

	tests := []struct {
		request string
		want    int
	}{
		{"status", 10},
		{"compile --target=linux", 600},
		{"unknown", 15},
		{"", 15},
	}

	for _, tt := range tests {
		t.Run(tt.request, func(t *testing.T) {
			got := GetActionTimeout(tt.request, whitelist)
			if got != tt.want {
				t.Errorf("GetActionTimeout(%q) = %d, want %d", tt.request, got, tt.want)
			}
		})
	}
}