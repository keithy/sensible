package main

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/keithy/sensible/pkg/sensible"
)

type Server struct {
	cfg      sensible.Config
	storage  sensible.TaskRepository
	executor sensible.Executor
}

func main() {
	cfg := sensible.LoadConfig()
	storage := sensible.NewStorage(cfg.TasksDir)
	executor := sensible.NewExeExecutor(cfg.ActionsDir)

	srv := &Server{
		cfg:      cfg,
		storage:  storage,
		executor: executor,
	}

	r := mux.NewRouter()
	r.HandleFunc("/sensible", srv.handleSensible)
	r.HandleFunc("/sensible/{id}", srv.handleTask)
	r.HandleFunc("/sensible/{id}/", srv.handleTaskChain)

	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("sensible-server listening on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func (s *Server) handleSensible(w http.ResponseWriter, r *http.Request) {
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
			request = strings.TrimSpace(string(body))
		}
	}

	if !s.checkAuth(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	field := r.URL.Query().Get("field")
	if field != "" {
		s.handleField(w, request, field)
		return
	}

	request = strings.TrimSpace(request)
	if request == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// Check if it's a task ID
	if task, err := s.storage.Load(request); err == nil && task != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(task)
		return
	}

	s.handleExecute(w, r, request, timeout)
}

func (s *Server) handleField(w http.ResponseWriter, request, field string) {
	task, err := s.storage.Load(request)
	if err == nil && task != nil {
		writeField(w, field, task)
		return
	}

	result := s.executor.Execute(request, 0)
	task = &sensible.Task{
		Status: result.Status,
		ExitCode: result.ExitCode,
		Stdout: result.Stdout,
		Stderr: result.Stderr,
		DurationMs: result.DurationMs,
	}
	if task.Status == "" {
		task.Status = "unknown"
	}
	writeField(w, field, task)
}

func writeField(w http.ResponseWriter, field string, task *sensible.Task) {
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

func (s *Server) handleTask(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		id = mux.Vars(r)["id"]
	}

	if !s.checkAuth(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	field := r.URL.Query().Get("field")
	if field != "" {
		if task, err := s.storage.Load(id); err == nil && task != nil {
			writeField(w, field, task)
		}
		return
	}

	task, err := s.storage.Load(id)
	if err != nil || task == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

func (s *Server) handleTaskChain(w http.ResponseWriter, r *http.Request) {
	parentID := mux.Vars(r)["id"]
	request := r.URL.Query().Get("request")

	if !s.checkAuth(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	parent, err := s.storage.Load(parentID)
	if err != nil || parent == nil {
		http.Error(w, "parent task not found", http.StatusNotFound)
		return
	}

	request = strings.TrimSpace(request)
	if request == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	parts := strings.Fields(request)
	if len(parts) == 0 {
		http.Error(w, "empty request", http.StatusBadRequest)
		return
	}
	action := parts[0]

	// Check whitelist
	found := false
	for _, a := range s.cfg.Whitelist {
		if a.Name == action {
			found = true
			break
		}
	}
	if !found {
		http.Error(w, "action not whitelisted", http.StatusForbidden)
		return
	}

	timeout := sensible.GetActionTimeout(request, s.cfg.Whitelist)

	// Create dependent task
	task := sensible.CreateDependentTask(parentID, action, request)

	// Check if parent is already complete
	if parent.Status == "success" || parent.Status == "failed" {
		result := s.executor.Execute(request, timeout)
		task.Status = result.Status
		task.ExitCode = result.ExitCode
		task.Stdout = result.Stdout
		task.Stderr = result.Stderr
		task.DurationMs = result.DurationMs
		task.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)

		s.storage.MoveToDone(task)
		// Delete from pending (already moved to done)
		s.storage.Delete(task.FileID)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(task)
		return
	}

	// Parent not complete - save as queued
	task.Status = "queued"
	if err := s.storage.Save(task); err != nil {
		http.Error(w, "failed to save task", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"id": task.ID, "file_id": task.FileID, "status": "queued", "depends_on": parentID})
}

func (s *Server) handleExecute(w http.ResponseWriter, r *http.Request, request string, timeout int) {
	if timeout == 0 {
		timeout = sensible.GetActionTimeout(request, s.cfg.Whitelist)
	}

	parts := strings.Fields(request)
	if len(parts) == 0 {
		http.Error(w, "empty request", http.StatusBadRequest)
		return
	}
	action := parts[0]

	var actionCfg sensible.ActionConfig
	found := false
	for _, a := range s.cfg.Whitelist {
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

	actionTimeout := actionCfg.Timeout
	if timeout > 0 && timeout < actionTimeout {
		actionTimeout = timeout
	}

	task := sensible.NewTask(action, request)

	const defaultSyncTimeout = 15 // seconds

	if actionTimeout <= defaultSyncTimeout {
		result := s.executor.Execute(request, actionTimeout)
		task.Status = result.Status
		task.ExitCode = result.ExitCode
		task.Stdout = result.Stdout
		task.Stderr = result.Stderr
		task.DurationMs = result.DurationMs
		task.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(task)
		return
	}

	// Long task, run async - save queued first
	if err := s.storage.Save(task); err != nil {
		http.Error(w, "failed to save task", http.StatusInternalServerError)
		return
	}

	go func() {
		result := s.executor.Execute(request, actionTimeout)
		task.Status = result.Status
		task.ExitCode = result.ExitCode
		task.Stdout = result.Stdout
		task.Stderr = result.Stderr
		task.DurationMs = result.DurationMs
		task.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)

		s.storage.MoveToDone(task)
		s.storage.Delete(task.FileID)

		// Trigger dependents of this task
		s.triggerDependents(task.ID)
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"id": task.ID, "file_id": task.FileID})
}

func (s *Server) triggerDependents(parentID string) {
	tasks, err := s.storage.ListPending()
	if err != nil {
		return
	}

	for _, task := range tasks {
		if task.DependsOn != parentID {
			continue
		}
		if task.Status != "queued" {
			continue
		}

		// Execute the dependent task
		timeout := sensible.GetActionTimeout(task.Request, s.cfg.Whitelist)
		result := s.executor.Execute(task.Request, timeout)
		task.Status = result.Status
		task.ExitCode = result.ExitCode
		task.Stdout = result.Stdout
		task.Stderr = result.Stderr
		task.DurationMs = result.DurationMs
		task.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)

		s.storage.MoveToDone(task)
		s.storage.Delete(task.FileID)

		// Recursively trigger any dependents
		s.triggerDependents(task.FileID)
	}
}

func (s *Server) checkAuth(r *http.Request) bool {
	if len(s.cfg.APIKeys) == 0 {
		return true
	}

	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return false
	}
	token := strings.TrimPrefix(auth, "Bearer ")

	for _, key := range s.cfg.APIKeys {
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

func init() {
	// Ensure directories exist
	cfg := sensible.LoadConfig()
	os.MkdirAll(filepath.Join(cfg.TasksDir, "pending"), 0755)
	os.MkdirAll(filepath.Join(cfg.TasksDir, "done"), 0755)
}