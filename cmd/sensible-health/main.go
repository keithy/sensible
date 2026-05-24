package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/keithy/sensible/pkg/sensible"
)

type HealthReport struct {
	Status       string   `json:"status"`
	Version      string   `json:"version"`
	TasksDir     string   `json:"tasks_dir"`
	PendingDir   string   `json:"pending_dir"`
	DoneDir      string   `json:"done_dir"`
	PendingCount int      `json:"pending_count"`
	DoneCount    int      `json:"done_count"`
	Errors       []string `json:"errors,omitempty"`
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "check" {
		checkHealth()
		return
	}

	field := ""
	if len(os.Args) > 1 && os.Args[1] != "health" {
		field = os.Args[1]
	}

	report := buildHealthReport()

	if field == "" {
		json.NewEncoder(os.Stdout).Encode(report)
		return
	}

	// Output specific field verbatim
	switch field {
	case "status":
		fmt.Print(report.Status)
	case "version":
		fmt.Print(report.Version)
	case "pending_count":
		fmt.Print(report.PendingCount)
	case "done_count":
		fmt.Print(report.DoneCount)
	case "tasks_dir":
		fmt.Print(report.TasksDir)
	default:
		fmt.Fprintf(os.Stderr, "Unknown field: %s\n", field)
		os.Exit(1)
	}
}

func checkHealth() {
	report := buildHealthReport()
	if report.Status == "healthy" {
		fmt.Println("pong")
	} else {
		for _, e := range report.Errors {
			fmt.Fprintln(os.Stderr, "ERROR:", e)
		}
		os.Exit(1)
	}
}

func buildHealthReport() *HealthReport {
	report := &HealthReport{
		Version: "1.0.0",
		Errors:  []string{},
	}

	cfg := sensible.LoadConfig()
	report.TasksDir = cfg.TasksDir
	report.PendingDir = filepath.Join(cfg.TasksDir, "pending")
	report.DoneDir = filepath.Join(cfg.TasksDir, "done")

	// Check pending dir
	if info, err := os.Stat(report.PendingDir); err != nil {
		report.Errors = append(report.Errors, "pending dir: "+err.Error())
	} else if !info.IsDir() {
		report.Errors = append(report.Errors, "pending dir not a directory")
	}

	// Check done dir
	if info, err := os.Stat(report.DoneDir); err != nil {
		report.Errors = append(report.Errors, "done dir: "+err.Error())
	} else if !info.IsDir() {
		report.Errors = append(report.Errors, "done dir not a directory")
	}

	// Count pending
	pendingFiles, _ := filepath.Glob(filepath.Join(report.PendingDir, "*.json"))
	report.PendingCount = len(pendingFiles)

	// Count done
	doneFiles, _ := filepath.Glob(filepath.Join(report.DoneDir, "*.json"))
	report.DoneCount = len(doneFiles)

	if len(report.Errors) == 0 {
		report.Status = "healthy"
	} else {
		report.Status = "unhealthy"
	}

	return report
}
