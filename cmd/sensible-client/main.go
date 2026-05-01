package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	switch os.Args[1] {
	case "do":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: sensible-client do <action> [args...]")
			os.Exit(1)
		}
		request := strings.Join(os.Args[2:], " ")
		if err := doAction(request); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}

	case "list":
		if err := listTasks(); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}

	case "status":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: sensible-client status <file_id>")
			os.Exit(1)
		}
		if err := showStatus(os.Args[2]); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}

	default:
		printUsage()
	}
}

func printUsage() {
	fmt.Println("Usage: sensible-client <command>")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  sensible-client do <action> [args...]  Execute action via HTTP API")
	fmt.Println("  sensible-client list                   List pending tasks")
	fmt.Println("  sensible-client status <file_id>       Check task status")
	fmt.Println("")
	fmt.Println("Environment:")
	fmt.Println("  SENSIBLE_HOST          API host:port (default: localhost:2222)")
	fmt.Println("  SENSIBLE_AUTH_HEADER   Authorization header value")
}

func getHost() string {
	if host := os.Getenv("SENSIBLE_HOST"); host != "" {
		return host
	}
	return "localhost:2222"
}

func getAuthHeader() string {
	return os.Getenv("SENSIBLE_AUTH_HEADER")
}

func doAction(request string) error {
	host := getHost()
	url := fmt.Sprintf("http://%s/sensible?request=%s", host, strings.ReplaceAll(request, " ", "%20"))

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return err
	}

	auth := getAuthHeader()
	if auth != "" {
		req.Header.Set("Authorization", "Bearer "+auth)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode == http.StatusAccepted {
		// Async task
		var result map[string]string
		json.Unmarshal(body, &result)
		fmt.Println("Async task queued")
		if id, ok := result["file_id"]; ok {
			fmt.Println("FileID:", id)
		}
		fmt.Println("")
		fmt.Println("Poll with: sensible-client status <file_id>")
	} else if resp.StatusCode == http.StatusOK {
		// Sync result - pretty print
		var task map[string]interface{}
		json.Unmarshal(body, &task)
		printTask(task)
	} else {
		fmt.Println(string(body))
	}

	return nil
}

func listTasks() error {
	host := getHost()
	url := fmt.Sprintf("http://%s/sensible?request=list-pending", host)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	auth := getAuthHeader()
	if auth != "" {
		req.Header.Set("Authorization", "Bearer "+auth)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if len(body) == 0 || string(body) == "null" {
		fmt.Println("No pending tasks")
		return nil
	}

	fmt.Println(string(body))
	return nil
}

func showStatus(fileID string) error {
	host := getHost()
	url := fmt.Sprintf("http://%s/sensible/%s", host, fileID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	auth := getAuthHeader()
	if auth != "" {
		req.Header.Set("Authorization", "Bearer "+auth)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("task not found: %s", fileID)
	}

	var task map[string]interface{}
	json.Unmarshal(body, &task)
	printTask(task)

	return nil
}

func printTask(task map[string]interface{}) {
	fields := []struct {
		key      string
		label    string
	}{
		{"id", "ID"},
		{"file_id", "FileID"},
		{"status", "Status"},
		{"request", "Request"},
		{"exit_code", "ExitCode"},
		{"duration_ms", "Duration"},
		{"timestamp", "Timestamp"},
		{"depends_on", "DependsOn"},
	}

	for _, f := range fields {
		if v, ok := task[f.key]; ok && v != nil && v != "" {
			fmt.Printf("%-12s %v\n", f.label+":", v)
		}
	}

	if v, ok := task["stdout"]; ok && v != nil && v != "" {
		fmt.Printf("Stdout:\n%s\n", v)
	}
	if v, ok := task["stderr"]; ok && v != nil && v != "" {
		fmt.Printf("Stderr:\n%s\n", v)
	}
}