package sensible

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Execline built-in commands (always allowed)
// Source: https://skarnet.org/software/execline/
var execlineBuiltins = map[string]bool{
	// Script parser
	"execlineb": true,
	// Process state control
	"cd": true, "posix-cd": true,
	"umask": true, "posix-umask": true,
	"emptyenv": true,
	"envfile": true,
	"export": true, "export-array": true,
	"unexport": true,
	"fdclose": true, "fdblock": true,
	"fdmove": true, "fdswap": true, "fdreserve": true,
	"redirfd": true, "piperw": true,
	"heredoc": true,
	"wait": true, "getcwd": true, "getpid": true,
	"exec": true, "tryexec": true,
	"exit": true, "trap": true,
	"withstdinas": true,
	// Basic block management
	"foreground": true, "background": true,
	"case": true,
	"if": true, "ifnot": true, "ifelse": true,
	"ifte": true, "ifthenelse": true,
	"backtick": true, "pipeline": true, "runblock": true,
	// Variable management
	"define": true, "importas": true,
	"elglob": true, "elgetpositionals": true,
	"multidefine": true, "multisubstitute": true,
	// Loops
	"forx": true, "forstdin": true, "forbacktickx": true,
	"loopwhilex": true,
	// Positional params
	"elgetopt": true, "shift": true, "dollarat": true,
	// Miscellaneous
	"eltest": true, "homeof": true,
	// Multicall
	"execline": true,
	// Provided scripts
	"execline-shell": true, "execline-startup": true,
}

// Builtins that execute content and need checking
var builtins = map[string]bool{
	// Executors - check content inside { }
	"foreground": true, "background": true,
	"if": true, "ifnot": true, "ifelse": true, "ifte": true, "ifthenelse": true,
	"forx": true, "forstdin": true, "forbacktickx": true, "loopwhilex": true,
	"exec": true, "tryexec": true,
	"pipeline": true,
	// backtick - checks content AFTER }
	"backtick": true,
}

// Config holds sensible configuration
type Config struct {
	Port     int
	TasksDir string
	KeysDir  string
	APIKeys  []string

	Whitelist []string // Regex patterns that are allowed (checked against block content)
	Blacklist []string // Regex patterns that are denied
	BuiltinBlacklist []string // Regex patterns to block specific builtins

	// Compiled regexes (runtime only)
	whitelistRe []*regexp.Regexp
	blacklistRe []*regexp.Regexp
	builtinBlacklistRe []*regexp.Regexp
}

// LoadConfig loads configuration from environment variables and config file
func LoadConfig() Config {
	cfg := Config{
		Port:     2222,
		KeysDir:   getEnv("SENSIBLE_KEYS_DIR", "/etc/sensible/keys"),
		TasksDir:   getEnv("SENSIBLE_TASKS_DIR", "/var/lib/sensible/tasks"),
		Whitelist:  []string{"^sensible"},
		Blacklist:  []string{},
	}

	// Load config file if present
	loadConfigFile(&cfg)

	// Compile regex patterns
	cfg.compileRe()

	// Load API keys
	keys, _ := filepath.Glob(filepath.Join(cfg.KeysDir, "*.pem"))
	for _, f := range keys {
		if key, err := os.ReadFile(f); err == nil {
			cfg.APIKeys = append(cfg.APIKeys, strings.TrimSpace(string(key)))
		}
	}

	return cfg
}

func (c *Config) compileRe() {
	for _, pattern := range c.Whitelist {
		if re, err := regexp.Compile(pattern); err == nil {
			c.whitelistRe = append(c.whitelistRe, re)
		}
	}
	for _, pattern := range c.Blacklist {
		if re, err := regexp.Compile(pattern); err == nil {
			c.blacklistRe = append(c.blacklistRe, re)
		}
	}
	for _, pattern := range c.BuiltinBlacklist {
		if re, err := regexp.Compile(pattern); err == nil {
			c.builtinBlacklistRe = append(c.builtinBlacklistRe, re)
		}
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func loadConfigFile(cfg *Config) {
	// Check CONFIG env var first, then common locations
	home := os.Getenv("HOME")
	if home == "" {
		home = "/root"
	}
	paths := []string{
		os.Getenv("SENSIBLE_CONFIG"),
		"/etc/sensible/config.json",
		"/etc/sensible/config.yaml",
		filepath.Join(home, ".config/sensible/config.json"),
		filepath.Join(home, ".config/sensible/config.yaml"),
	}

	for _, path := range paths {
		if path == "" {
			continue
		}
		if data, err := os.ReadFile(path); err == nil {
			// Try JSON first
			if strings.HasSuffix(path, ".json") || !strings.Contains(path, ".") {
				if err := json.Unmarshal(data, cfg); err == nil {
					return
				}
			}
		}
	}
}

// GetConfigFilePath returns the path of the first found config file
func GetConfigFilePath() string {
	home := os.Getenv("HOME")
	if home == "" {
		home = "/root"
	}
	paths := []string{
		os.Getenv("SENSIBLE_CONFIG"),
		"/etc/sensible/config.json",
		"/etc/sensible/config.yaml",
		filepath.Join(home, ".config/sensible/config.json"),
		filepath.Join(home, ".config/sensible/config.yaml"),
	}

	for _, path := range paths {
		if path == "" {
			continue
		}
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

// GetConfigFileContent returns the raw content of the config file
func GetConfigFileContent() (string, error) {
	path := GetConfigFilePath()
	if path == "" {
		return "", fmt.Errorf("no config file found")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// IsAllowed checks if a script action is permitted
// Uses brace-aware parsing to handle nesting
func (c *Config) IsAllowed(script string) bool {
	script = strings.TrimSpace(script)
	if script == "" {
		return true
	}

	// Parse and check recursively
	return c.checkScript(script)
}

// checkScript recursively checks script content
func (c *Config) checkScript(script string) bool {
	script = strings.TrimSpace(script)
	if script == "" {
		return true
	}

	lines := splitLines(script)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}
		command := parts[0]

		// Check if it's an execline builtin that needs checking
		if builtins[command] {
			// Check builtinBlacklist first
			for _, re := range c.builtinBlacklistRe {
				if re.MatchString(command) {
					return false // blocked
				}
			}

			// Get content to check - braces or rest of line
			var content string
			if command == "backtick" {
				content = extractAfterBraces(line)
			} else {
				content = extractInsideBraces(line)
				if content == "" {
					// No braces - check the rest of the line (after command)
					rest := strings.TrimSpace(line[len(command):])
					if rest != "" {
						content = rest
					}
				}
			}
			if content != "" && !c.checkScript(content) {
				return false
			}
			continue
		}

		// Non-builtin or non-executing builtin - check against whitelist/blacklist
		if !c.checkContent(line) {
			return false
		}
	}

	return true
}

// extractInsideBraces extracts content inside the first { } block
// Handles nested braces by counting depth
func extractInsideBraces(line string) string {
	// Find the first {
	start := strings.Index(line, "{")
	if start == -1 {
		return ""
	}

	// Find matching } by counting brace depth
	depth := 0
	for i := start + 1; i < len(line); i++ {
		if line[i] == '{' {
			depth++
		} else if line[i] == '}' {
			if depth == 0 {
				// Found matching closing brace
				return strings.TrimSpace(line[start+1 : i])
			}
			depth--
		}
	}
	return ""
}

// extractAfterBraces extracts content after the closing }
func extractAfterBraces(line string) string {
	// Find the last }
	idx := strings.LastIndex(line, "}")
	if idx == -1 {
		return ""
	}
	return strings.TrimSpace(line[idx+1:])
}

// checkContent validates a line against whitelist and blacklist
func (c *Config) checkContent(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return true
	}

	// Check whitelist - if matches, allowed
	for _, re := range c.whitelistRe {
		if re.MatchString(line) {
			return true
		}
	}

	// Check blacklist - if matches, denied
	for _, re := range c.blacklistRe {
		if re.MatchString(line) {
			return false
		}
	}

	// Not in whitelist, not in blacklist - allowed by default
	return true
}

// splitLines splits script on newlines
func splitLines(script string) []string {
	lines := strings.Split(script, "\n")
	var result []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}