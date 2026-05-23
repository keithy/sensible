package main

import (
	"fmt"
	"os"

	"github.com/keithy/sensible/pkg/sensible"
)

func main() {
	cfg := sensible.LoadConfig()
	storage := sensible.NewStorage(cfg.TasksDir)

	tasks, err := storage.ListPending()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(tasks) == 0 {
		fmt.Println("No pending tasks")
		return
	}

	fmt.Println("Pending tasks:")
	for _, t := range tasks {
		dep := ""
		if t.DependsOn != "" {
			dep = fmt.Sprintf(" (depends on: %s)", t.DependsOn)
		}
		fmt.Printf("  %s  %s%s\n", t.FileID, t.Request, dep)
	}
}