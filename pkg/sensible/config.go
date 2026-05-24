package sensible

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Config holds sensible configuration
type Config struct {
	Port       int
	ActionsDir string
	KeysDir    string
	TasksDir   string
	Whitelist  []string
	Blacklist  []string
	APIKeys    []string

	// Compiled regexes (runtime only)
	whitelistRe []*regexp.Regexp
	blacklistRe []*regexp.Regexp
}

// LoadConfig loads configuration from environment variables and config file
func LoadConfig() Config {
	cfg := Config{
		Port:       2222,
		ActionsDir: getEnv("SENSIBLE_ACTIONS_DIR", "/var/lib/sensible/actions"),
		KeysDir:    getEnv("SENSIBLE_KEYS_DIR", "/etc/sensible/keys"),
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

// IsAllowed checks if a script action is permitted
// Uses regex patterns only - user specifies full pattern in config
func (c *Config) IsAllowed(script string) bool {
	script = strings.TrimSpace(script)

	// Check whitelist regex first
	for _, re := range c.whitelistRe {
		if re.MatchString(script) {
			return true
		}
	}

	// Check blacklist regex
	for _, re := range c.blacklistRe {
		if re.MatchString(script) {
			return false
		}
	}

	// Empty whitelist means all allowed (unless blacklisted)
	if len(c.Whitelist) == 0 && len(c.whitelistRe) == 0 {
		return true
	}

	return false
}