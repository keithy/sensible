package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/keithy/sensible/pkg/sensible"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	cfg := sensible.LoadConfig()
	storage := sensible.NewStorage(cfg.TasksDir)
	executor := sensible.NewExeExecutor(cfg.ActionsDir)

	switch os.Args[1] {
	case "do":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: sensible-queue do <action> [args...]")
			os.Exit(1)
		}
		request := strings.Join(os.Args[2:], " ")
		if err := doAction(cfg, storage, executor, request); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}

	case "worker":
		if err := runWorker(cfg, storage, executor); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}

	case "list":
		if err := listTasks(storage); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}

	case "status":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: sensible-queue status <file_id>")
			os.Exit(1)
		}
		if err := showStatus(storage, os.Args[2]); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}

	default:
		printUsage()
	}
}

func printUsage() {
	fmt.Println("Usage: sensible-queue <command>")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  sensible-queue do <action> [args...]  Execute action")
	fmt.Println("  sensible-queue worker                 Run as background worker")
	fmt.Println("  sensible-queue list                   List pending tasks")
	fmt.Println("  sensible-queue status <id>            Check task status")
	fmt.Println("")
	fmt.Println("Environment:")
	fmt.Println("  SENSIBLE_TASKS_DIR   Task storage directory")
	fmt.Println("  SENSIBLE_ACTIONS_DIR Action scripts directory")
}

func doAction(cfg sensible.Config, storage sensible.TaskRepository, executor sensible.Executor, request string) error {
	parts := strings.Fields(request)
	if len(parts) == 0 {
		return fmt.Errorf("empty request")
	}
	action := parts[0]

	// Check whitelist
	found := false
	for _, a := range cfg.Whitelist {
		if a.Name == action {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("action %q not whitelisted", action)
	}

	timeout := sensible.GetActionTimeout(request, cfg.Whitelist)
	result := executor.Execute(request, timeout)

	fmt.Printf("Status: %s\n", result.Status)
	fmt.Printf("Exit code: %d\n", result.ExitCode)
	if result.Stdout != "" {
		fmt.Printf("Stdout:\n%s\n", result.Stdout)
	}
	if result.Stderr != "" {
		fmt.Printf("Stderr:\n%s\n", result.Stderr)
	}
	fmt.Printf("Duration: %dms\n", result.DurationMs)

	return nil
}

func runWorker(cfg sensible.Config, storage sensible.TaskRepository, executor sensible.Executor) error {
	fmt.Println("Starting worker, processing pending tasks...")

	for {
		tasks, err := storage.ListPending()
		if err != nil {
			return err
		}

		if len(tasks) == 0 {
			time.Sleep(1 * time.Second)
			continue
		}

		// Find oldest pending task that has no unmet dependency
		var task *sensible.Task
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
			task = t
			break
		}

		if task == nil {
			time.Sleep(1 * time.Second)
			continue
		}

		fmt.Printf("Executing: %s (request: %q)\n", task.FileID, task.Request)

		timeout := sensible.GetActionTimeout(task.Request, cfg.Whitelist)
		result := executor.Execute(task.Request, timeout)

		task.Status = result.Status
		task.ExitCode = result.ExitCode
		task.Stdout = result.Stdout
		task.Stderr = result.Stderr
		task.DurationMs = result.DurationMs
		task.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)

		if err := storage.MoveToDone(task); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to move to done: %v\n", err)
		}
		if err := storage.Delete(task.FileID); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to delete from pending: %v\n", err)
		}

		fmt.Printf("Completed: %s (status: %s)\n", task.FileID, task.Status)
	}
}

func listTasks(storage sensible.TaskRepository) error {
	tasks, err := storage.ListPending()
	if err != nil {
		return err
	}

	if len(tasks) == 0 {
		fmt.Println("No pending tasks")
		return nil
	}

	fmt.Println("Pending tasks:")
	for _, t := range tasks {
		dep := ""
		if t.DependsOn != "" {
			dep = fmt.Sprintf(" (depends on: %s)", t.DependsOn)
		}
		fmt.Printf("  %s  %s%s\n", t.FileID, t.Request, dep)
	}
	return nil
}

func showStatus(storage sensible.TaskRepository, fileID string) error {
	task, err := storage.Load(fileID)
	if err != nil {
		return err
	}
	if task == nil {
		return fmt.Errorf("task not found: %s", fileID)
	}

	fmt.Printf("ID:        %s\n", task.ID)
	fmt.Printf("FileID:    %s\n", task.FileID)
	fmt.Printf("Status:    %s\n", task.Status)
	fmt.Printf("Request:   %s\n", task.Request)
	fmt.Printf("ExitCode:  %d\n", task.ExitCode)
	fmt.Printf("Duration:  %dms\n", task.DurationMs)
	fmt.Printf("Timestamp: %s\n", task.Timestamp)
	if task.DependsOn != "" {
		fmt.Printf("DependsOn: %s\n", task.DependsOn)
	}
	if task.Stdout != "" {
		fmt.Printf("Stdout:    %s\n", task.Stdout)
	}
	if task.Stderr != "" {
		fmt.Printf("Stderr:    %s\n", task.Stderr)
	}
	return nil
}