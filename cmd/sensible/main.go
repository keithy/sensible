package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	subcommand := os.Args[1]

	// Get directory of this executable
	exePath, err := os.Executable()
	if err != nil {
		exePath = os.Args[0]
	}
	exeDir := filepath.Dir(exePath)

	// Check if subcommand binary exists in same directory
	subPath := filepath.Join(exeDir, subcommand)
	if _, err := os.Stat(subPath); err == nil {
		run(subPath, os.Args[2:])
		return
	}

	// Fallback: look for binary with sensible- prefix
	prefixedName := "sensible-" + subcommand
	prefixedPath := filepath.Join(exeDir, prefixedName)
	if _, err := os.Stat(prefixedPath); err == nil {
		run(prefixedPath, os.Args[2:])
		return
	}

	// Not found
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
	fmt.Println("Usage: sensible <command> [args...]")
	fmt.Println("")
	fmt.Println("Available commands:")

	// Get directory of this executable
	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)

	// List available binaries
	entries, err := os.ReadDir(exeDir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			// Show sensible-* commands and this wrapper
			if name == "sensible" {
				continue
			}
			if len(name) > 9 && name[:9] == "sensible-" {
				fmt.Printf("  %s\n", name[9:])
			}
		}
	}

	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  sensible queue do compile")
	fmt.Println("  sensible server")
	fmt.Println("  sensible client status")
}
