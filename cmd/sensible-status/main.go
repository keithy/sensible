package main

import (
	"fmt"
	"os"

	"github.com/keithy/sensible/pkg/sensible"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: sensible-status <file_id>")
		os.Exit(1)
	}

	fileID := os.Args[1]

	cfg := sensible.LoadConfig()
	storage := sensible.NewStorage(cfg.TasksDir)

	task, err := storage.Load(fileID)
	if err != nil {
		// Try in done/
		cfg.TasksDir += "/done"
		storage = sensible.NewStorage(cfg.TasksDir)
		task, err = storage.Load(fileID)
	}

	if err != nil || task == nil {
		fmt.Fprintf(os.Stderr, "Task not found: %s\n", fileID)
		os.Exit(1)
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
}