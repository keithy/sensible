package main

import (
	"fmt"
	"os"

	"github.com/keithy/sensible/pkg/sensible"
)

func main() {
	cfg := sensible.LoadConfig()
	storage := sensible.NewStorage(cfg.TasksDir)
	executor := sensible.NewExeExecutor()

	// Loop: find and process all ready tasks until none left
	for {
		task := findReadyTask(storage)
		if task == nil {
			fmt.Println("sensible-consume: no ready tasks")
			break
		}
		processTask(task, cfg, storage, executor)
	}
}

func findReadyTask(storage sensible.TaskRepository) *sensible.Task {
	tasks, err := storage.ListPending()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing tasks: %v\n", err)
		return nil
	}

	for _, t := range tasks {
		if t.Status == "queued" {
			// No dependsOn check needed - runNext handles ordering
			return t
		}
	}
	return nil
}

func processTask(task *sensible.Task, cfg sensible.Config, storage sensible.TaskRepository, executor sensible.Executor) {
	fmt.Printf("sensible-consume: executing %s (%s)\n", task.FileID, task.Request)

	timeout := sensible.GetActionTimeout(task.Request, cfg.Whitelist)
	result := executor.Execute(task.Request, timeout)

	// Update task with result
	task.Status = result.Status
	task.ExitCode = result.ExitCode
	task.Stdout = result.Stdout
	task.Stderr = result.Stderr
	task.DurationMs = result.DurationMs

	// If task failed and has runNext, fail the chain
	if result.Status == "failed" && task.RunNext != "" {
		failChain(task.RunNext, storage)
	}

	// Move to done/
	if err := storage.MoveToDone(task); err != nil {
		fmt.Fprintf(os.Stderr, "Error moving to done: %v\n", err)
	}

	// Delete from pending/
	if err := storage.Delete(task.FileID); err != nil {
		fmt.Fprintf(os.Stderr, "Error deleting from pending: %v\n", err)
	}

	fmt.Printf("sensible-consume: %s -> %s (exit=%d, %dms)\n",
		task.FileID, task.Status, task.ExitCode, task.DurationMs)
}

// failChain recursively marks all downstream tasks as failed
func failChain(fileID string, storage sensible.TaskRepository) {
	next, err := storage.Load(fileID)
	if err != nil || next == nil {
		return
	}

	fmt.Printf("sensible-consume: failing chain -> %s (upstream failed)\n", next.FileID)

	next.Status = "failed"
	next.Stderr = "upstream task failed"
	storage.MoveToDone(next)
	storage.Delete(next.FileID)

	// Continue failing the chain
	if next.RunNext != "" {
		failChain(next.RunNext, storage)
	}
}