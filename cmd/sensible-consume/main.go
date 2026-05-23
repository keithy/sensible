package main

import (
	"fmt"
	"os"

	"github.com/keithy/sensible/pkg/sensible"
)

func main() {
	cfg := sensible.LoadConfig()
	storage := sensible.NewStorage(cfg.TasksDir)
	executor := sensible.NewExeExecutor(cfg.ActionsDir)

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
		if t.Status != "queued" {
			continue
		}
		if t.DependsOn != "" {
			parent, err := storage.Load(t.DependsOn)
			if err != nil || parent == nil {
				continue
			}
			if parent.Status != "success" && parent.Status != "failed" {
				continue
			}
		}
		return t
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