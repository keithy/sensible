package sensible

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Allowed environment variables for execline execution
var allowedEnv = map[string]bool{
	"PATH":        true,
	"HOME":        true,
	"USER":        true,
	"LOGNAME":     true,
	"SHELL":       true,
	"TERM":        true,
	"LANG":        true,
	"TZ":          true,
	"DISPLAY":    true,
	"TMPDIR":      true,
	"PWD":         true,
	"OLDPWD":      true,
	"EDITOR":      true,
	"PAGER":       true,
	"HTTP_PROXY":  true,
	"http_proxy":  true,
	"HTTPS_PROXY": true,
	"https_proxy": true,
	"NO_PROXY":    true,
	"no_proxy":    true,
}

// Sensible-controlled environment variables (matched via SENSIBLE_* prefix)
// Note: These are allowlisted via prefix match, this map is for documentation
var sensibleEnv = map[string]bool{
	"SENSIBLE_TASKS_DIR": true,
	"SENSIBLE_CONFIG":    true,
}

// Blocked environment variables (security)
var blockedEnv = map[string]bool{
	"LD_PRELOAD":       true,
	"LD_LIBRARY_PATH":  true,
	"LD_AUDIT":         true,
	"LD_DEBUG":         true,
}

// Execute runs a task request via execlineb and returns the result
func (e *ExeExecutor) Execute(request string, timeout int) *Result {
	if timeout == 0 {
		timeout = 300
	}

	tmp, err := os.CreateTemp("", "sensible-*.sh")
	if err != nil {
		return &Result{Status: "failed", Stderr: fmt.Sprintf("temp: %v", err)}
	}
	tmpPath := tmp.Name()
	tmp.WriteString(request)
	tmp.Close()
	defer os.Remove(tmpPath)

	env := buildSafeEnv()

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
			return &Result{Status: "timeout", Stderr: "timeout", DurationMs: duration}
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

	return &Result{
		Status:     status,
		ExitCode:   exitCode,
		Stdout:     outBuf.String(),
		Stderr:     errBuf.String(),
		DurationMs: duration,
	}
}

// buildSafeEnv constructs a safe environment for execline execution
func buildSafeEnv() []string {
	var env []string

	// PATH: include standard locations plus common installation paths
	// Note: PATH is safe to pass - it's just search paths for commands
	// We use literal paths since execlineb doesn't expand variables
	home := os.Getenv("HOME")
	if home == "" {
		home = "/root"
	}
	env = append(env, "PATH=/usr/local/bin:/usr/bin:/bin:"+home+"/.local/bin")

	// Add allowed vars from parent environment
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]

		// Skip blocked vars
		if blockedEnv[key] {
			continue
		}

		// Skip sensitive patterns
		if strings.Contains(key, "API_KEY") ||
			strings.Contains(key, "SECRET") ||
			strings.Contains(key, "PASSWORD") ||
			strings.Contains(key, "TOKEN") {
			continue
		}

		// Allow if in allowlist
		if allowedEnv[key] {
			env = append(env, e)
			continue
		}

		// Allow XDG_* vars
		if strings.HasPrefix(key, "XDG_") {
			env = append(env, e)
			continue
		}

		// Allow LC_* locale vars
		if strings.HasPrefix(key, "LC_") {
			env = append(env, e)
			continue
		}
	}

	return env
}

// GetActionTimeout returns the timeout for an action from the whitelist
func GetActionTimeout(request string, whitelist []string) int {
	parts := strings.Fields(request)
	if len(parts) == 0 {
		return 15
	}
	action := parts[0]
	for _, allowed := range whitelist {
		if action == allowed {
			return 300 // default timeout for whitelisted actions
		}
	}
	return 15
}