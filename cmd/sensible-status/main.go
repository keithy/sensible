package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/keithy/sensible/pkg/sensible"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: sensible-status <file_id> [field]")
		fmt.Fprintln(os.Stderr, "  field: status, exit_code, stdout, stderr, duration_ms, timestamp, depends_on, request")
		os.Exit(1)
	}

	fileID := os.Args[1]
	field := ""

	if len(os.Args) > 2 {
		field = os.Args[2]
	}

	cfg := sensible.LoadConfig()
	storage := sensible.NewStorage(cfg.TasksDir)

	task, err := storage.Load(fileID)
	if err != nil {
		cfg.TasksDir += "/done"
		storage = sensible.NewStorage(cfg.TasksDir)
		task, err = storage.Load(fileID)
	}

	if err != nil || task == nil {
		fmt.Fprintf(os.Stderr, "Task not found: %s\n", fileID)
		os.Exit(1)
	}

	if field == "" {
		json.NewEncoder(os.Stdout).Encode(task)
		return
	}

	// Output specific field verbatim
	switch field {
	case "status":
		fmt.Print(task.Status)
	case "exit_code":
		fmt.Print(task.ExitCode)
	case "stdout":
		fmt.Print(task.Stdout)
	case "stderr":
		fmt.Print(task.Stderr)
	case "duration_ms":
		fmt.Print(task.DurationMs)
	case "timestamp":
		fmt.Print(task.Timestamp)
	case "request":
		fmt.Print(task.Request)
	case "id":
		fmt.Print(task.ID)
	default:
		fmt.Fprintf(os.Stderr, "Unknown field: %s\n", field)
		os.Exit(1)
	}
}