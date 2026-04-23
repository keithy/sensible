package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

// Config holds sensible configuration
type Config struct {
	Port       int
	ActionsDir string
	KeysDir    string
	TasksDir   string
	Whitelist  []ActionConfig
}

// ActionConfig describes an allowed action
type ActionConfig struct {
	Name    string
	Timeout int
}

// Task represents a stored task result
type Task struct {
	ID         string `json:"id"`
	Status     string `json:"status"`
	ExitCode   int    `json:"exit_code,omitempty"`
	Stdout     string `json:"stdout,omitempty"`
	Stderr     string `json:"stderr,omitempty"`
	DurationMs int64  `json:"duration_ms,omitempty"`
	Timestamp  string `json:"timestamp"`
}

var (
	cfg       Config
	apiKeys   []string
	tasks     = map[string]*Task{}
	tasksMu   sync.RWMutex
	waiting   = map[string][]string{}
	waitingMu sync.RWMutex
)

const defaultTimeout = 15 // seconds

func main() {
	if err := loadConfig(); err != nil {
		log.Fatalf("config: %v", err)
	}

	r := mux.NewRouter()
	r.HandleFunc("/sensible", handleSensible)
	r.HandleFunc("/sensible/{id}", handleTask)
	r.HandleFunc("/sensible/{id}/", handleTaskChain)

	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("sensible listening on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func loadConfig() error {
	cfg.Port = 8443
	cfg.ActionsDir = getEnv("SENSIBLE_ACTIONS_DIR", "/var/lib/sensible/actions")
	cfg.KeysDir = getEnv("SENSIBLE_KEYS_DIR", "/etc/sensible/keys")
	cfg.TasksDir = getEnv("SENSIBLE_TASKS_DIR", "/var/lib/sensible/tasks")

	os.MkdirAll(cfg.TasksDir, 0755)

	// Default whitelist
	cfg.Whitelist = []ActionConfig{
		{Name: "status", Timeout: 10},
		{Name: "restart", Timeout: 60},
		{Name: "compile", Timeout: 600},
		{Name: "update", Timeout: 300},
		{Name: "test", Timeout: 300},
	}

	// Load API keys
	if keys, err := filepath.Glob(filepath.Join(cfg.KeysDir, "*.pem")); err == nil {
		for _, f := range keys {
			if key, err := os.ReadFile(f); err == nil {
				apiKeys = append(apiKeys, strings.TrimSpace(string(key)))
			}
		}
	}

	return nil
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func handleSensible(w http.ResponseWriter, r *http.Request) {
	// Get request from query or body
	var request string
	var timeout int

	switch r.Method {
	case "GET":
		request = r.URL.Query().Get("request")
		if t := r.URL.Query().Get("timeout"); t != "" {
			timeout, _ = strconv.Atoi(t)
		}
	case "POST":
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		var m map[string]interface{}
		if err := json.Unmarshal(body, &m); err == nil {
			request = getString(m, "request")
			if t, ok := m["timeout"]; ok {
				timeout, _ = strconv.Atoi(fmt.Sprintf("%v", t))
			}
		} else {
			// Treat body as plain request
			request = strings.TrimSpace(string(body))
		}
	}

	// Check API key
	if !checkAuth(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Handle field extraction only
	field := r.URL.Query().Get("field")
	if field != "" {
		handleField(w, request, field)
		return
	}

	// Parse request
	request = strings.TrimSpace(request)
	if request == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// Check if it's a task ID (look up stored result)
	if task := getTask(request); task != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(task)
		return
	}

	// Execute request
	handleExecute(w, r, request, timeout)
}

func handleField(w http.ResponseWriter, request, field string) {
	// If request looks like a task ID, return that field
	if task := getTask(request); task != nil {
		writeField(w, field, task)
		return
	}

	// Otherwise execute and return field from result
	task := executeTask(request, 0)
	if task.Status == "" {
		task.Status = "unknown"
	}
	writeField(w, field, task)
}

func writeField(w http.ResponseWriter, field string, task *Task) {
	var val interface{}
	switch field {
	case "id":
		val = task.ID
	case "status":
		val = task.Status
	case "exit_code":
		if task.ExitCode == 0 && task.Status == "success" {
			val = 0
		} else if task.Status == "queued" {
			val = nil
		} else {
			val = task.ExitCode
		}
	case "stdout":
		val = task.Stdout
	case "stderr":
		val = task.Stderr
	case "duration_ms":
		val = task.DurationMs
	case "timestamp":
		val = task.Timestamp
	default:
		val = ""
	}
	fmt.Fprint(w, val)
}

func handleTask(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		id = mux.Vars(r)["id"]
	}

	// Check API key
	if !checkAuth(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	field := r.URL.Query().Get("field")
	if field != "" {
		if task := getTask(id); task != nil {
			writeField(w, field, task)
		}
		return
	}

	task := getTask(id)
	if task == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

func handleTaskChain(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	request := r.URL.Query().Get("request")

	// Check API key
	if !checkAuth(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Wait for the task to complete
	waitForTask(id)

	// Now execute the new request
	timeout := defaultTimeout
	if t := r.URL.Query().Get("timeout"); t != "" {
		timeout, _ = strconv.Atoi(t)
	}

	handleExecute(w, r, request, timeout)
}

func handleExecute(w http.ResponseWriter, r *http.Request, request string, timeout int) {
	if timeout == 0 {
		timeout = 0 // Never wait
	} else if timeout < 0 {
		timeout = defaultTimeout
	}

	// Parse action name (first word)
	parts := strings.Fields(request)
	if len(parts) == 0 {
		http.Error(w, "empty request", http.StatusBadRequest)
		return
	}
	action := parts[0]

	// Find action config
	var actionCfg ActionConfig
	found := false
	for _, a := range cfg.Whitelist {
		if a.Name == action {
			actionCfg = a
			found = true
			break
		}
	}
	if !found {
		http.Error(w, "action not whitelisted", http.StatusForbidden)
		return
	}

	// Determine sync vs async
	actionTimeout := actionCfg.Timeout
	if timeout == 0 {
		// Client explicitly wants async
	} else if timeout > 0 {
		// Client timeout
		if actionTimeout <= timeout {
			actionTimeout = timeout
		}
	}

	// Execute
	id := fmt.Sprintf("%s-%d", action, time.Now().UnixMilli())

	// Short enough? Run sync
	if actionTimeout <= defaultTimeout || timeout > 0 {
		task := executeTask(request, actionTimeout)
		task.ID = id
		tasksMu.Lock()
		tasks[id] = task
		tasksMu.Unlock()
		saveTask(task)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(task)
		return
	}

	// Long task, run async
	task := &Task{
		ID:        id,
		Status:    "queued",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	tasksMu.Lock()
	tasks[id] = task
	tasksMu.Unlock()
	saveTask(task)

	// Execute in background
	go func() {
		result := executeTask(request, actionTimeout)
		tasksMu.Lock()
		tasksMu.Unlock()
		task.Status = result.Status
		task.ExitCode = result.ExitCode
		task.Stdout = result.Stdout
		task.Stderr = result.Stderr
		task.DurationMs = result.DurationMs
		task.Timestamp = time.Now().UTC().Format(time.RFC3339)
		saveTask(task)

		// Wake up any waiting tasks
		waitingMu.Lock()
		if waiters, ok := waiting[id]; ok {
			for _, w := range waiters {
				if t, exists := tasks[w]; exists {
					t.Status = "ready"
				}
			}
		}
		delete(waiting, id)
		waitingMu.Unlock()
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"id": id})
}

func executeTask(request string, timeout int) *Task {
	if timeout == 0 {
		timeout = 300
	}

	// Write to temp file
	tmp, err := os.CreateTemp("", "sensible-*.sh")
	if err != nil {
		return &Task{Status: "failed", Stderr: fmt.Sprintf("temp: %v", err)}
	}
	tmpPath := tmp.Name()
	tmp.WriteString(request)
	tmp.Close()
	defer os.Remove(tmpPath)

	// Build environment
	env := os.Environ()
	env = append(env, "PATH="+cfg.ActionsDir+":/usr/bin:/bin")

	// Execute
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "execlineb", tmpPath)
	cmd.Env = env

	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err = cmd.Run()
	duration := time.Since(start).Milliseconds()

	exitCode := 0
	if err != nil {
		if ctx.Err() != nil {
			return &Task{Status: "timeout", Stderr: "timeout", DurationMs: duration}
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	status := "success"
	if exitCode != 0 {
		status = "failed"
	}

	return &Task{
		Status:     status,
		ExitCode:   exitCode,
		Stdout:     outBuf.String(),
		Stderr:     errBuf.String(),
		DurationMs: duration,
	}
}

func getTask(id string) *Task {
	tasksMu.RLock()
	defer tasksMu.RUnlock()

	if task, ok := tasks[id]; ok {
		return task
	}

	// Try loading from disk
	path := filepath.Join(cfg.TasksDir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var task Task
	if err := json.Unmarshal(data, &task); err != nil {
		return nil
	}

	tasks[id] = &task
	return &task
}

func saveTask(task *Task) {
	path := filepath.Join(cfg.TasksDir, task.ID+".json")
	data, _ := json.MarshalIndent(task, "", "  ")
	os.WriteFile(path, data, 0644)
}

func waitForTask(id string) {
	for {
		task := getTask(id)
		if task == nil {
			return
		}
		if task.Status == "success" || task.Status == "failed" {
			return
		}
		if task.Status == "queued" || task.Status == "running" {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		return
	}
}

func checkAuth(r *http.Request) bool {
	if len(apiKeys) == 0 {
		return true // No keys configured
	}

	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return false
	}
	token := strings.TrimPrefix(auth, "Bearer ")

	for _, key := range apiKeys {
		if subtle.ConstantTimeCompare([]byte(token), []byte(key)) == 1 {
			return true
		}
	}
	return false
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
