package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/keithy/sensible/pkg/sensible"
)

func main() {
	cfg := sensible.LoadConfig()
	storage := sensible.NewStorage(cfg.TasksDir)
	executor := sensible.NewExeExecutor(cfg.ActionsDir)

	fmt.Printf("sensible-consume: watching %s/pending/\n", cfg.TasksDir)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nsensible-consume: shutting down...")
		os.Exit(0)
	}()

	for {
		tasks, err := storage.ListPending()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error listing tasks: %v\n", err)
			time.Sleep(1 * time.Second)
			continue
		}

		// Find oldest ready task
		var task *sensible.Task
		for _, t := range tasks {
			if t.Status != "queued" {
				continue
			}
			// Check dependencies
			if t.DependsOn != "" {
				parent, err := storage.Load(t.DependsOn)
				if err != nil || parent == nil {
					continue
				}
				if parent.Status != "success" && parent.Status != "failed" {
					continue
				}
			}
			task = t
			break
		}

		if task == nil {
			time.Sleep(1 * time.Second)
			continue
		}

		fmt.Printf("sensible-consume: executing %s (%s)\n", task.FileID, task.Request)

		timeout := sensible.GetActionTimeout(task.Request, cfg.Whitelist)
		start := time.Now()
		result := executor.Execute(task.Request, timeout)
		duration := time.Since(start).Milliseconds()

		// Update task with result
		task.Status = result.Status
		task.ExitCode = result.ExitCode
		task.Stdout = result.Stdout
		task.Stderr = result.Stderr
		task.DurationMs = duration
		task.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)

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
}