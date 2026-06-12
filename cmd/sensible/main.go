package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	args := os.Args[1:]

	// If no arguments given but SSH_ORIGINAL_COMMAND is set (via sshd command=),
	// parse it to get the command the client tried to run
	if len(args) == 0 {
		if origCmd := os.Getenv("SSH_ORIGINAL_COMMAND"); origCmd != "" {
			args = parseArgs(origCmd)
			// SSH_ORIGINAL_COMMAND includes our binary name (e.g., "sensible do ...")
			// Strip it if args[0] matches our binary name
			exeName := filepath.Base(os.Args[0])
			if len(args) > 0 && args[0] == exeName {
				args = args[1:]
			}
		}
	}

	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		printUsage()
		return
	}

	subcommand := args[0]

	// Hidden command to list commands as JSON (for sensible-info)
	if subcommand == "--commands" {
		listCommands()
		return
	}

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

	// Search path order:
	// 1. exeDir/<subcommand>
	// 2. exeDir/build/<prefix>-<subcommand>
	// 3. exeDir/<prefix>-<subcommand>
	// 4. sibling lib/<prefix>-<subcommand>
	// 5. PATH (standard)

	// Check if subcommand exists directly
	subPath := filepath.Join(exeDir, subcommand)
	if _, err := os.Stat(subPath); err == nil {
		run(subPath, args[1:])
		return
	}

	// Check ./build/ for subcommands (repo structure)
	buildDir := filepath.Join(exeDir, "build")
	prefixedName := prefix + "-" + subcommand
	subPath = filepath.Join(buildDir, prefixedName)
	if _, err := os.Stat(subPath); err == nil {
		run(subPath, args[1:])
		return
	}

	// Look for prefix-subcommand in exeDir
	prefixedPath := filepath.Join(exeDir, prefixedName)
	if _, err := os.Stat(prefixedPath); err == nil {
		run(prefixedPath, args[1:])
		return
	}

	// Check sibling lib directory (for local installs)
	// If exeDir is ~/.local/bin, check ~/.local/lib/sensible/
	siblingLib := filepath.Join(exeDir, "..", "lib", "sensible")
	prefixedPath = filepath.Join(siblingLib, prefixedName)
	if _, err := os.Stat(prefixedPath); err == nil {
		run(prefixedPath, args[1:])
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
	fmt.Printf("Use '%s <command> --help' for command-specific help\n", exeName)
	fmt.Println("")
	fmt.Printf("Examples:\n")
	fmt.Printf("  %s info\n", exeName)
	fmt.Printf("  %s info status\n", exeName)
	fmt.Printf("  %s do compile /path/to/script\n", exeName)
	fmt.Printf("  %s server\n", exeName)
}

func listCommands() {
	exePath, err := os.Executable()
	if err != nil {
		fmt.Println("[]")
		return
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
				commands = append(commands, name[prefixLen:])
			}
		}
	}
	
	fmt.Print("[")
	for i, cmd := range commands {
		if i > 0 {
			fmt.Print(", ")
		}
		fmt.Printf("\"%s\"", cmd)
	}
	fmt.Println("]")
}

// parseArgs parses a command string into arguments, respecting quotes.
// Used to parse SSH_ORIGINAL_COMMAND when invoked via sshd's command= option.
func parseArgs(cmd string) []string {
	var args []string
	var current strings.Builder
	inQuote := false
	quoteChar := ' '

	for i := 0; i < len(cmd); i++ {
		c := cmd[i]

		if !inQuote && (c == '"' || c == '\'') {
			inQuote = true
			quoteChar = rune(c)
			continue
		}

		if inQuote && rune(c) == quoteChar {
			inQuote = false
			continue
		}

		if !inQuote && c == ' ' {
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
			continue
		}

		current.WriteByte(c)
	}

	if current.Len() > 0 {
		args = append(args, current.String())
	}

	return args
}
