package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// Config holds application configuration
type Config struct {
	Port           string
	Host           string
	DatabasePath   string
	WorkspacesDir  string
	KeysDir        string
	LogsDir        string
	DockerNetwork  string
	TraefikNetwork string
	Domain         string
	LetsEncryptEmail string
	TraefikDashboard bool
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	// Try to load .env file (ignore error if not found)
	_ = godotenv.Load()

	cfg := &Config{
		Port:             getEnv("PORT", "3000"),
		Host:             getEnv("HOST", "0.0.0.0"),
		DatabasePath:     getEnv("DATABASE_PATH", "/opt/atmosphere/atmosphere.db"),
		WorkspacesDir:    getEnv("WORKSPACES_DIR", "/opt/atmosphere/workspaces"),
		KeysDir:          getEnv("KEYS_DIR", "/opt/atmosphere/keys"),
		LogsDir:          getEnv("LOGS_DIR", "/opt/atmosphere/logs"),
		DockerNetwork:    getEnv("DOCKER_NETWORK", "atmosphere"),
		TraefikNetwork:   getEnv("TRAEFIK_NETWORK", "traefik"),
		Domain:           getEnv("DOMAIN", ""),
		LetsEncryptEmail: getEnv("LETSENCRYPT_EMAIL", "admin@example.com"),
		TraefikDashboard: getEnv("TRAEFIK_DASHBOARD", "false") == "true",
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks if configuration is valid
func (c *Config) Validate() error {
	if c.Port == "" {
		return fmt.Errorf("PORT cannot be empty")
	}
	if c.DatabasePath == "" {
		return fmt.Errorf("DATABASE_PATH cannot be empty")
	}
	if c.WorkspacesDir == "" {
		return fmt.Errorf("WORKSPACES_DIR cannot be empty")
	}
	if c.KeysDir == "" {
		return fmt.Errorf("KEYS_DIR cannot be empty")
	}
	if c.LogsDir == "" {
		return fmt.Errorf("LOGS_DIR cannot be empty")
	}
	if c.DockerNetwork == "" {
		return fmt.Errorf("DOCKER_NETWORK cannot be empty")
	}
	if c.TraefikNetwork == "" {
		return fmt.Errorf("TRAEFIK_NETWORK cannot be empty")
	}
	return nil
}

// getEnv retrieves an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
