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
	cfg := sensible.LoadConfig()
	switch field {
	case "status":
		if checkPending() == nil && checkDone() == nil {
			return "OK"
		}
		return "unhealthy"
	case "version":
		return "1.0.0"
	case "port":
		return fmt.Sprintf("%d", cfg.Port)
	case "tasksDir":
		return cfg.TasksDir
	case "keysDir":
		return cfg.KeysDir
	case "whitelist":
		return fmt.Sprintf("%v", cfg.Whitelist)
	case "blacklist":
		return fmt.Sprintf("%v", cfg.Blacklist)
	case "commands":
		return fmt.Sprintf("%v", []string{"do", "consume", "list", "status", "server", "client", "info"})
	case "config":
		content, err := sensible.GetConfigFileContent()
		if err != nil {
			return ""
		}
		// Return parsed config or raw if invalid JSON
		var parsed interface{}
		if json.Unmarshal([]byte(content), &parsed) == nil {
			if m, ok := parsed.(map[string]interface{}); ok {
				return fmt.Sprintf("%v", m)
			}
		}
		return content
	case "pendingCount":
		return countFiles(filepath.Join(getTasksDir(), "pending"), "*.json")
	case "doneCount":
		return countFiles(filepath.Join(getTasksDir(), "done"), "*.json")
	case "pending_count":
		return countFiles(filepath.Join(getTasksDir(), "pending"), "*.json")
	case "done_count":
		return countFiles(filepath.Join(getTasksDir(), "done"), "*.json")
	case "tasks_dir":
		return cfg.TasksDir
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
		"commands":  []string{"do", "consume", "list", "status", "server", "client", "info"},
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
		"commands":  []string{"do", "consume", "list", "status", "server", "client", "info"},
	}
}

func buildHealthReport() *struct {
	Status       string   `json:"status"`
	Version      string   `json:"version"`
	Port         int      `json:"port"`
	TasksDir     string   `json:"tasksDir"`
	KeysDir      string   `json:"keysDir"`
	Whitelist    []string `json:"whitelist"`
	Blacklist    []string `json:"blacklist"`
	Commands     []string `json:"commands"`
	PendingCount int      `json:"pendingCount"`
	DoneCount    int      `json:"doneCount"`
	Errors       []string `json:"errors"`
	Config       interface{} `json:"config"`
} {
	cfg := sensible.LoadConfig()
	configContent, _ := sensible.GetConfigFileContent()
	var configData interface{}
	json.Unmarshal([]byte(configContent), &configData)
	report := &struct {
		Status       string   `json:"status"`
		Version      string   `json:"version"`
		Port         int      `json:"port"`
		TasksDir     string   `json:"tasksDir"`
		KeysDir      string   `json:"keysDir"`
		Whitelist    []string `json:"whitelist"`
		Blacklist    []string `json:"blacklist"`
		Commands     []string `json:"commands"`
		PendingCount int      `json:"pendingCount"`
		DoneCount    int      `json:"doneCount"`
		Errors       []string `json:"errors"`
		Config       interface{} `json:"config"`
	}{
		Version:    "1.0.0",
		Port:       cfg.Port,
		TasksDir:   cfg.TasksDir,
		KeysDir:    cfg.KeysDir,
		Whitelist:  cfg.Whitelist,
		Blacklist:  cfg.Blacklist,
		Commands:   []string{"do", "consume", "list", "status", "server", "client", "info"},
		Errors:     []string{},
		Config:     configData,
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