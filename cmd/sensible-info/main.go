package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/keithy/sensible/pkg/sensible"
)

func main() {
	field := ""

	// Handle "sensible health [field]" (wrapper call)
	if len(os.Args) > 2 {
		field = os.Args[2]
	} else if len(os.Args) > 1 && os.Args[1] != "health" {
		// "sensible-health status" - direct call with field
		field = os.Args[1]
	}

	// For JSON output, build struct then marshal
	if field == "" {
		report := buildHealthReport()
		jsonOut, _ := json.Marshal(report)
		fmt.Println(string(jsonOut))
		return
	}

	// For specific fields, compute directly without building full struct
	// Support paths like "config.tasksDir"
	if strings.Contains(field, ".") {
		report := buildHealthReport()
		jsonOut, _ := json.Marshal(report)
		var data map[string]interface{}
		json.Unmarshal(jsonOut, &data)
		parts := strings.Split(field, ".")
		for _, part := range parts {
			if v, ok := data[part].(map[string]interface{}); ok {
				data = v
			} else if v, ok := data[part].(string); ok {
				fmt.Print(v)
				return
			} else if v, ok := data[part].(int); ok {
				fmt.Print(v)
				return
			} else if v, ok := data[part].(float64); ok {
				fmt.Print(int(v))
				return
			} else if v, ok := data[part].(bool); ok {
				fmt.Print(v)
				return
			}
		}
		return
	}

	// Single field - use getFieldValue
	value := getFieldValue(field)
	fmt.Print(value)
}

func getFieldValue(field string) string {
	switch field {
	case "status":
		if checkPending() == nil && checkDone() == nil {
			return "OK"
		}
		return "unhealthy"
	case "version":
		return "1.0.0"
	case "pending_count":
		return countFiles(filepath.Join(getTasksDir(), "pending"), "*.json")
	case "done_count":
		return countFiles(filepath.Join(getTasksDir(), "done"), "*.json")
	case "tasks_dir":
		return configMap()["tasksDir"].(string)
	case "config":
		return configJson()
	case "errors":
		if err := checkPending(); err != nil {
			return "1"
		} else if err := checkDone(); err != nil {
			return "1"
		}
		return "0"
	default:
		report := buildHealthReport()
		jsonOut, _ := json.Marshal(report)
		return string(jsonOut)
	}
}

func getTasksDir() string {
	cfg := sensible.LoadConfig()
	return cfg.TasksDir
}

func checkPending() error {
	pendingDir := filepath.Join(getTasksDir(), "pending")
	if info, err := os.Stat(pendingDir); err != nil {
		return err
	} else if !info.IsDir() {
		return fmt.Errorf("not a directory")
	}
	return nil
}

func checkDone() error {
	doneDir := filepath.Join(getTasksDir(), "done")
	if info, err := os.Stat(doneDir); err != nil {
		return err
	} else if !info.IsDir() {
		return fmt.Errorf("not a directory")
	}
	return nil
}

func countFiles(dir, pattern string) string {
	files, _ := filepath.Glob(filepath.Join(dir, pattern))
	return fmt.Sprintf("%d", len(files))
}

func configJson() string {
	cfg := sensible.LoadConfig()
	cfgMap := map[string]interface{}{
		"port":      cfg.Port,
		"tasksDir":  cfg.TasksDir,
		"keysDir":   cfg.KeysDir,
		"whitelist": cfg.Whitelist,
		"blacklist": cfg.Blacklist,
	}
	cfgJson, _ := json.Marshal(cfgMap)
	return string(cfgJson)
}

func configMap() map[string]interface{} {
	cfg := sensible.LoadConfig()
	return map[string]interface{}{
		"port":      cfg.Port,
		"tasksDir":  cfg.TasksDir,
		"keysDir":   cfg.KeysDir,
		"whitelist": cfg.Whitelist,
		"blacklist": cfg.Blacklist,
	}
}

func buildHealthReport() *struct {
	Status       string `json:"status"`
	Version      string `json:"version"`
	PendingCount int    `json:"pendingCount"`
	DoneCount    int    `json:"doneCount"`
	Errors       []string `json:"errors"`
	Config       map[string]interface{} `json:"config"`
} {
	report := &struct {
		Status       string `json:"status"`
		Version      string `json:"version"`
		PendingCount int    `json:"pendingCount"`
		DoneCount    int    `json:"doneCount"`
		Errors       []string `json:"errors"`
		Config       map[string]interface{} `json:"config"`
	}{
		Version: "1.0.0",
		Errors:  []string{},
		Config: configMap(),
	}

	if err := checkPending(); err != nil {
		report.Errors = append(report.Errors, "pending dir: "+err.Error())
	}

	if err := checkDone(); err != nil {
		report.Errors = append(report.Errors, "done dir: "+err.Error())
	}

	pendingCount, _ := strconv.Atoi(countFiles(filepath.Join(getTasksDir(), "pending"), "*.json"))
	doneCount, _ := strconv.Atoi(countFiles(filepath.Join(getTasksDir(), "done"), "*.json"))
	report.PendingCount = pendingCount
	report.DoneCount = doneCount

	if len(report.Errors) == 0 {
		report.Status = "OK"
	} else {
		report.Status = "unhealthy"
	}

	return report
}