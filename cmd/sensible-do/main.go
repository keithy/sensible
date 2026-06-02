package main

import (
	"fmt"
	"os"
	"time"

	"github.com/keithy/sensible/pkg/sensible"
)

// parseArgs handles "||" as separate argument for fallback:
// "build" "||" "build-alt" → "ifelse { build } { } { build-alt }"
func parseArgs(args []string) []string {
	var result []string
	var pendingScript string
	var pendingWrapped string

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if arg == "||" {
			if pendingScript == "" {
				fmt.Fprintln(os.Stderr, "Error: || without preceding script")
				os.Exit(1)
			}
			pendingWrapped = fmt.Sprintf("ifelse { %s } { } {", pendingScript)
			pendingScript = ""
		} else if arg == "&&" {
			// && is ignored - chain logic handles success continuation via run_next
			// Just save pending script and continue
			if pendingScript != "" {
				result = append(result, pendingScript)
				pendingScript = ""
			}
		} else if pendingWrapped != "" {
			// We're in fallback mode - arg completes it
			pendingWrapped = fmt.Sprintf("%s %s }", pendingWrapped, arg)
			result = append(result, pendingWrapped)
			pendingWrapped = ""
		} else if pendingScript != "" {
			// Previous script exists but no || - save it and start new
			result = append(result, pendingScript)
			pendingScript = arg
		} else {
			pendingScript = arg
		}
	}

	// Save any remaining script
	if pendingWrapped != "" {
		result = append(result, pendingWrapped)
	} else if pendingScript != "" {
		result = append(result, pendingScript)
	}

	return result
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: sensible-do <script> [<script>...]")
		fmt.Fprintln(os.Stderr, "  Chain: sensible-do \"echo hello\" \"echo world\"")
		fmt.Fprintln(os.Stderr, "  Each script is execlineb content, chained in order.")
		fmt.Fprintln(os.Stderr, "  Supports: \"cmd1\" \"||\" \"cmd2\" → ifelse fallback.")
		os.Exit(1)
	}

	// Parse args, combining "||" operators
	scripts := parseArgs(os.Args[1:])

	cfg := sensible.LoadConfig()
	storage := sensible.NewStorage(cfg.TasksDir)

	var prevTask *sensible.Task

	for i, script := range scripts {
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