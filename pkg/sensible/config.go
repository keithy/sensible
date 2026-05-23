package sensible

import (
	"os"
	"path/filepath"
	"strings"
)

// Config holds sensible configuration
type Config struct {
	Port       int
	ActionsDir string
	KeysDir    string
	TasksDir   string
	Whitelist  []ActionConfig
	APIKeys    []string
}

// ActionConfig describes an allowed action
type ActionConfig struct {
	Name    string
	Timeout int
}

// LoadConfig loads configuration from environment variables
func LoadConfig() Config {
	cfg := Config{
		Port:       2222,
		ActionsDir: getEnv("SENSIBLE_ACTIONS_DIR", "/var/lib/sensible/actions"),
		KeysDir:    getEnv("SENSIBLE_KEYS_DIR", "/etc/sensible/keys"),
		TasksDir:   getEnv("SENSIBLE_TASKS_DIR", "/var/lib/sensible/tasks"),
		Whitelist: []ActionConfig{
			{Name: "status", Timeout: 10},
			{Name: "restart", Timeout: 60},
			{Name: "compile", Timeout: 600},
			{Name: "update", Timeout: 300},
			{Name: "test", Timeout: 300},
		},
	}

	// Load API keys
	keys, _ := filepath.Glob(filepath.Join(cfg.KeysDir, "*.pem"))
	for _, f := range keys {
		if key, err := os.ReadFile(f); err == nil {
			cfg.APIKeys = append(cfg.APIKeys, strings.TrimSpace(string(key)))
		}
	}

	return cfg
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}