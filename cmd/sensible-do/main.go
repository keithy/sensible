package main

import (
	"fmt"
	"os"
	"time"

	"github.com/keithy/sensible/pkg/sensible"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: sensible-do <script> [<script>...]")
		fmt.Fprintln(os.Stderr, "  Chain: sensible-do \"echo hello\" \"echo world\"")
		fmt.Fprintln(os.Stderr, "  Each script is execlineb content, chained in order.")
		os.Exit(1)
	}

	cfg := sensible.LoadConfig()
	storage := sensible.NewStorage(cfg.TasksDir)

	var prevTask *sensible.Task

	for i, script := range os.Args[1:] {
		if script == "" {
			fmt.Fprintln(os.Stderr, "Error: empty script")
			os.Exit(1)
		}

		if !cfg.IsAllowed(script) {
			fmt.Fprintf(os.Stderr, "Error: script %q not allowed\n", script)
			os.Exit(1)
		}

		// Create task
		timestamp := time.Now().UTC().Format(time.RFC3339Nano)
		action := fmt.Sprintf("script-%d", i+1)
		fileID := fmt.Sprintf("%s-%s", timestamp, action)
		task := &sensible.Task{
			ID:        action,
			FileID:    fileID,
			Request:   script,
			Status:    "queued",
			Timestamp: timestamp,
		}

		// If there's a previous task, set its RunNext to this task
		if prevTask != nil {
			prevTask.RunNext = fileID
			if err := storage.Save(prevTask); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving task: %v\n", err)
				os.Exit(1)
			}
		}

		// Save current task (if first task, no prevTask to update)
		if err := storage.Save(task); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving task: %v\n", err)
			os.Exit(1)
		}

		fmt.Println(fileID)
		prevTask = task
	}
}