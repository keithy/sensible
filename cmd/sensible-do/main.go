package main

import (
	"fmt"
	"os"
	"time"

	"github.com/keithy/sensible/pkg/sensible"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: sensible-do <action> [args...]")
		fmt.Fprintln(os.Stderr, "  Or: sensible-do <action> --depends-on <file_id>")
		os.Exit(1)
	}

	cfg := sensible.LoadConfig()
	storage := sensible.NewStorage(cfg.TasksDir)

	// Parse args
	var action string
	var dependsOn string
	var args []string

	i := 1
	for i < len(os.Args) {
		arg := os.Args[i]
		if arg == "--depends-on" && i+1 < len(os.Args) {
			dependsOn = os.Args[i+1]
			i += 2
		} else if action == "" {
			action = arg
			i++
		} else {
			args = append(args, arg)
			i++
		}
	}

	if action == "" {
		fmt.Fprintln(os.Stderr, "Usage: sensible-do <action> [args...]")
		os.Exit(1)
	}

	// Check whitelist
	found := false
	for _, a := range cfg.Whitelist {
		if a.Name == action {
			found = true
			break
		}
	}
	if !found {
		fmt.Fprintf(os.Stderr, "Error: action %q not whitelisted\n", action)
		os.Exit(1)
	}

	// Build request string
	request := action
	if len(args) > 0 {
		for _, a := range args {
			request += " " + a
		}
	}

	// Create task
	timestamp := time.Now().UTC().Format(time.RFC3339Nano)
	fileID := fmt.Sprintf("%s-%s", timestamp, action)
	task := &sensible.Task{
		ID:        action,
		FileID:    fileID,
		Request:   request,
		Status:    "queued",
		Timestamp: timestamp,
	}
	if dependsOn != "" {
		task.DependsOn = dependsOn
	}

	// Save to pending/
	if err := storage.Save(task); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving task: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(fileID)
}