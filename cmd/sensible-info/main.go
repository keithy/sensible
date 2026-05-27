package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/keithy/sensible/pkg/sensible"
)

func main() {
	// Handle help flags
	for _, arg := range os.Args[1:] {
		if arg == "--help" || arg == "-h" {
			printUsage()
			return
		}
	}

	field := ""

	// Handle "sensible info [field]" (wrapper call)
	if len(os.Args) > 2 {
		field = os.Args[2]
	} else if len(os.Args) > 1 && os.Args[1] != "info" {
		// "sensible-info status" - direct call with field
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

func discoverCommands() []string {
	// Try to get commands from the wrapper
	// Wrapper is same binary but called without subcommand args
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		exeName := filepath.Base(exePath)
		
		// Determine wrapper name from our own name
		// If we're "acme-info", wrapper is "acme"
		prefix := exeName
		if strings.HasPrefix(prefix, "sensible-") {
			prefix = "sensible"
		}
		prefix = strings.TrimSuffix(prefix, "-wrapper")
		wrapperPath := filepath.Join(exeDir, prefix)
		
		cmd := exec.Command(wrapperPath, "--commands")
		output, err := cmd.Output()
		if err == nil {
			var commands []string
			if json.Unmarshal(output, &commands) == nil {
				return commands
			}
		}
	}
	
	// Fallback: scan executable directory
	exePath, err = os.Executable()
	if err != nil {
		return []string{}
	}
	exeDir := filepath.Dir(exePath)
	exeName := filepath.Base(exePath)

	prefix := exeName
	if strings.HasPrefix(prefix, "sensible-") {
		prefix = "sensible"
	}
	prefix = strings.TrimSuffix(prefix, "-wrapper")
	prefixLen := len(prefix) + 1

	commands := []string{}
	entries, err := os.ReadDir(exeDir)
	if err != nil {
		return commands
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if name == exeName {
			continue
		}
		if len(name) > prefixLen && name[:prefixLen] == prefix+"-" {
			commands = append(commands, name[prefixLen:])
		}
	}
	return commands
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
		return fmt.Sprintf("%v", discoverCommands())
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
		Commands:   discoverCommands(),
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

func getWrapperName() string {
	exePath, err := os.Executable()
	if err != nil {
		return "sensible"
	}
	exeName := filepath.Base(exePath)
	prefix := exeName
	if strings.HasPrefix(prefix, "sensible-") {
		prefix = "sensible"
	}
	prefix = strings.TrimSuffix(prefix, "-wrapper")
	return prefix
}

func printUsage() {
	wrapperName := getWrapperName()
	fmt.Printf("%s-info - Display configuration and status\n", wrapperName)
	fmt.Println("")
	fmt.Printf("Usage:\n")
	fmt.Printf("  %s info              Show full JSON report\n", wrapperName)
	fmt.Printf("  %s info <field>      Show specific field value\n", wrapperName)
	fmt.Printf("  %s info <path.to.val> Show nested value (e.g., config.port)\n", wrapperName)
	fmt.Println("")
	fmt.Println("Fields:")
	fmt.Println("  status       - OK or unhealthy")
	fmt.Println("  version      - sensible version")
	fmt.Println("  port         - server port")
	fmt.Println("  tasksDir     - tasks directory")
	fmt.Println("  keysDir      - keys directory")
	fmt.Println("  whitelist    - allowed script patterns")
	fmt.Println("  blacklist    - denied script patterns")
	fmt.Println("  commands     - available commands")
	fmt.Println("  pendingCount - pending tasks count")
	fmt.Println("  doneCount    - completed tasks count")
	fmt.Println("  errors       - error count (0 or 1)")
	fmt.Println("  config        - raw config file content")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  sensible info")
	fmt.Println("  sensible info status")
	fmt.Println("  sensible info config")
	fmt.Println("  sensible info config.port")
}