package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

// Config holds sensible configuration
type Config struct {
	Port       int
	ActionsDir string
	KeysDir    string
	Whitelist  []ActionConfig
}

// ActionConfig describes an allowed action
type ActionConfig struct {
	Name       string
	ArgsSchema map[string]string
	Timeout    int
}

// TaskRequest is the incoming task JSON
type TaskRequest struct {
	Request string `json:"request"`
	Timeout int    `json:"timeout,omitempty"`
}

// TaskResponse is the task result JSON
type TaskResponse struct {
	ID         string `json:"id"`
	Request    string `json:"request"`
	Status     string `json:"status"`
	ExitCode   int    `json:"exit_code,omitempty"`
	Reason     string `json:"reason,omitempty"`
	Stdout     string `json:"stdout,omitempty"`
	Stderr     string `json:"stderr,omitempty"`
	DurationMs int64  `json:"duration_ms,omitempty"`
	Timestamp  string `json:"timestamp"`
}

var cfg Config
var apiKeys []string

func main() {
	if err := loadConfig(); err != nil {
		log.Fatalf("config: %v", err)
	}

	r := mux.NewRouter()
	r.HandleFunc("/v1/health", healthHandler)
	r.HandleFunc("/v1/actions", withAuth(actionsHandler))
	r.HandleFunc("/v1/tasks", withAuth(tasksHandler)).Methods("POST")
	r.HandleFunc("/v1/tasks/{id}", withAuth(taskGetHandler)).Methods("GET")

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

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func actionsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cfg.Whitelist)
}

func tasksHandler(w http.ResponseWriter, r *http.Request) {
	var req TaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	id := fmt.Sprintf("task-%d", time.Now().UnixMilli())
	ts := time.Now().UTC().Format(time.RFC3339)

	// Whitelist check: extract action name (first word)
	parts := strings.Fields(req.Request)
	if len(parts) == 0 {
		sendResponse(w, &TaskResponse{
			ID:        id,
			Request:   req.Request,
			Status:    "rejected",
			Reason:    "EMPTY_REQUEST",
			Timestamp: ts,
		})
		return
	}
	action := parts[0]
	allowed := false
	var actionCfg ActionConfig
	for _, a := range cfg.Whitelist {
		if a.Name == action {
			allowed = true
			actionCfg = a
			break
		}
	}
	if !allowed {
		sendResponse(w, &TaskResponse{
			ID:        id,
			Request:   req.Request,
			Status:    "rejected",
			Reason:    "ACTION_NOT_WHITELISTED",
			Timestamp: ts,
		})
		return
	}

	// Args validation
	args := parts[1:]
	for name, pattern := range actionCfg.ArgsSchema {
		if pattern == "" {
			continue
		}
		for _, arg := range args {
			if strings.HasPrefix(arg, "--"+name+"=") {
				val := strings.SplitN(arg, "=", 2)[1]
				if matched, _ := regexp.MatchString(pattern, val); !matched {
					sendResponse(w, &TaskResponse{
						ID:     id,
						Status: "rejected",
						Reason: fmt.Sprintf("INVALID_ARG_%s", strings.ToUpper(name)),
					})
					return
				}
			}
		}
	}

	// Execute
	timeout := req.Timeout
	if timeout == 0 {
		timeout = actionCfg.Timeout
	}

	started := time.Now()
	stdout, stderr, exitCode := execute(req.Request, timeout)
	duration := time.Since(started).Milliseconds()

	status := "success"
	if exitCode != 0 {
		status = "failed"
	}

	sendResponse(w, &TaskResponse{
		ID:         id,
		Request:    req.Request,
		Status:     status,
		ExitCode:   exitCode,
		Stdout:     stdout,
		Stderr:     stderr,
		DurationMs: duration,
		Timestamp:  ts,
	})
}

func taskGetHandler(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func execute(request string, timeout int) (stdout, stderr string, exitCode int) {
	// Write to temp file for execlineb
	// request is already a valid command string like "compile --target=linux"
	tmp, err := os.CreateTemp("", "sensible-*.sh")
	if err != nil {
		return "", fmt.Sprintf("temp: %v", err), 1
	}
	tmpPath := tmp.Name()
	tmp.WriteString(request)
	tmp.Close()
	defer os.Remove(tmpPath)

	// Build environment
	env := os.Environ()
	env = append(env, "PATH="+cfg.ActionsDir+":/usr/bin:/bin")

	// Execute via execlineb
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "execlineb", tmpPath)
	cmd.Env = env

	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err = cmd.Run()
	if ctx.Err() != nil {
		return outBuf.String(), "timeout", 124
	}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	return outBuf.String(), errBuf.String(), exitCode
}

func withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		token := strings.TrimPrefix(auth, "Bearer ")

		valid := false
		for _, key := range apiKeys {
			if subtle.ConstantTimeCompare([]byte(token), []byte(key)) == 1 {
				valid = true
				break
			}
		}
		if !valid {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		next(w, r)
	}
}

func sendResponse(w http.ResponseWriter, resp *TaskResponse) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
