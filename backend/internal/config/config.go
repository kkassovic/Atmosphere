package config

import (
	"fmt"
	"os"
	"strings"

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
	TemplatesDir   string
	Domain         string
	LetsEncryptEmail string
	TraefikDashboard bool

	// S3 Backup Storage Configuration (optional)
	S3Endpoint   string
	S3Bucket     string
	S3Region     string
	S3AccessKey  string
	S3SecretKey  string
	S3PathPrefix string
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
		TemplatesDir:     getEnv("TEMPLATES_DIR", "templates/apps"),
		Domain:           getEnv("DOMAIN", ""),
		LetsEncryptEmail: getEnv("LETSENCRYPT_EMAIL", "admin@example.com"),
		TraefikDashboard: getEnv("TRAEFIK_DASHBOARD", "false") == "true",
		// S3 Configuration (optional)
		S3Endpoint:   getEnv("S3_ENDPOINT", ""),
		S3Bucket:     getEnv("S3_BUCKET", ""),
		S3Region:     getEnv("S3_REGION", ""),
		S3AccessKey:  getEnv("S3_ACCESS_KEY", ""),
		S3SecretKey:  getEnv("S3_SECRET_KEY", ""),
		S3PathPrefix: sanitizeS3PathPrefix(getEnv("S3_PATH_PREFIX", "atmosphere-backups")),
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
	if c.TemplatesDir == "" {
		return fmt.Errorf("TEMPLATES_DIR cannot be empty")
	}
	return nil
}

// IsS3Enabled checks if S3 backup storage is configured
func (c *Config) IsS3Enabled() bool {
	return c.S3Endpoint != "" && c.S3Bucket != "" && 
	       c.S3AccessKey != "" && c.S3SecretKey != ""
}

// getEnv retrieves an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func sanitizeS3PathPrefix(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	prefix = strings.ReplaceAll(prefix, "\\", "/")
	prefix = strings.TrimLeft(prefix, "-/")
	prefix = strings.TrimRight(prefix, "/")
	if prefix == "" {
		return "atmosphere-backups"
	}
	return prefix
}
