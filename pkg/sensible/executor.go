package sensible

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

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

	env := os.Environ()
	env = append(env, "PATH="+e.ActionsDir+":/usr/bin:/bin")

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

// GetActionTimeout returns the timeout for an action from the whitelist
func GetActionTimeout(request string, whitelist []ActionConfig) int {
	parts := strings.Fields(request)
	if len(parts) == 0 {
		return 15
	}
	action := parts[0]
	for _, a := range whitelist {
		if a.Name == action {
			return a.Timeout
		}
	}
	return 15
}