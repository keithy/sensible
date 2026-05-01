package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	subcommand := os.Args[1]

	exePath, err := os.Executable()
	if err != nil {
		exePath = os.Args[0]
	}
	exeDir := filepath.Dir(exePath)
	exeName := filepath.Base(exePath)

	// Determine prefix from our own name
	// "sensible" wrapper delegates to "sensible-queue", "sensible-server"
	// "acme" wrapper delegates to "acme-queue", "acme-server"
	prefix := exeName
	if strings.HasPrefix(prefix, "sensible-") {
		prefix = "sensible"
	}
	prefix = strings.TrimSuffix(prefix, "-wrapper")

	// Check if subcommand exists directly
	subPath := filepath.Join(exeDir, subcommand)
	if _, err := os.Stat(subPath); err == nil {
		run(subPath, os.Args[2:])
		return
	}

	// Look for prefix-subcommand
	prefixedName := prefix + "-" + subcommand
	prefixedPath := filepath.Join(exeDir, prefixedName)
	if _, err := os.Stat(prefixedPath); err == nil {
		run(prefixedPath, os.Args[2:])
		return
	}

	fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", subcommand)
	printUsage()
}

func run(path string, args []string) {
	cmd := exec.Command(path, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func printUsage() {
	exePath, _ := os.Executable()
	exeName := filepath.Base(exePath)

	prefix := exeName
	if strings.HasPrefix(prefix, "sensible-") {
		prefix = "sensible"
	}
	prefix = strings.TrimSuffix(prefix, "-wrapper")
	prefixLen := len(prefix) + 1

	fmt.Printf("Usage: %s <command> [args...]\n", exeName)
	fmt.Println("")
	fmt.Println("Available commands:")

	exeDir := filepath.Dir(exePath)
	entries, err := os.ReadDir(exeDir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if name == exeName {
				continue
			}
			if len(name) > prefixLen && name[:prefixLen] == prefix+"-" {
				fmt.Printf("  %s\n", name[prefixLen:])
			}
		}
	}

	fmt.Println("")
	fmt.Printf("Examples:\n")
	fmt.Printf("  %s queue do compile\n", exeName)
	fmt.Printf("  %s server\n", exeName)
	fmt.Printf("  %s client status\n", exeName)
}
