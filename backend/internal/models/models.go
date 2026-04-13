package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// App represents a deployed application
type App struct {
	ID             int64             `json:"id"`
	Name           string            `json:"name"`
	DeploymentType string            `json:"deployment_type"` // "github" or "manual"
	BuildType      string            `json:"build_type"`      // "dockerfile" or "compose"
	Status         string            `json:"status"`          // "stopped", "running", "building", "failed"
	Domain         string            `json:"domain,omitempty"`
	EnvVars        EnvVars           `json:"env_vars,omitempty"`
	GitHubRepo     string            `json:"github_repo,omitempty"`
	GitHubBranch   string            `json:"github_branch,omitempty"`
	GitHubSubdir   string            `json:"github_subdir,omitempty"`
	DockerfilePath string            `json:"dockerfile_path,omitempty"`
	ComposePath    string            `json:"compose_path,omitempty"`
	Port           int               `json:"port,omitempty"` // Container port to expose
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
	LastDeployedAt *time.Time        `json:"last_deployed_at,omitempty"`
}

// DeploymentLog represents a deployment attempt
type DeploymentLog struct {
	ID        int64     `json:"id"`
	AppID     int64     `json:"app_id"`
	Status    string    `json:"status"` // "success", "failed", "in_progress"
	Log       string    `json:"log"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
}

// EnvVars is a custom type for environment variables stored as JSON
type EnvVars map[string]string

// Scan implements sql.Scanner for EnvVars
func (e *EnvVars) Scan(value interface{}) error {
	if value == nil {
		*e = make(EnvVars)
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		*e = make(EnvVars)
		return nil
	}

	if len(bytes) == 0 {
		*e = make(EnvVars)
		return nil
	}

	return json.Unmarshal(bytes, e)
}

// Value implements driver.Valuer for EnvVars
func (e EnvVars) Value() (driver.Value, error) {
	if e == nil {
		return "{}", nil
	}
	return json.Marshal(e)
}

// CreateAppRequest represents the request to create a new app
type CreateAppRequest struct {
	Name           string  `json:"name"`
	DeploymentType string  `json:"deployment_type"` // "github" or "manual"
	BuildType      string  `json:"build_type"`      // "dockerfile" or "compose"
	Domain         string  `json:"domain,omitempty"`
	EnvVars        EnvVars `json:"env_vars,omitempty"`
	GitHubRepo     string  `json:"github_repo,omitempty"`
	GitHubBranch   string  `json:"github_branch,omitempty"`
	GitHubSubdir   string  `json:"github_subdir,omitempty"`
	DeploymentKey  string  `json:"deployment_key,omitempty"` // SSH private key for GitHub
	DockerfilePath string  `json:"dockerfile_path,omitempty"`
	ComposePath    string  `json:"compose_path,omitempty"`
	Port           int     `json:"port,omitempty"`
}

// UpdateAppRequest represents the request to update an app
type UpdateAppRequest struct {
	Domain         *string  `json:"domain,omitempty"`
	EnvVars        *EnvVars `json:"env_vars,omitempty"`
	GitHubBranch   *string  `json:"github_branch,omitempty"`
	GitHubSubdir   *string  `json:"github_subdir,omitempty"`
	DockerfilePath *string  `json:"dockerfile_path,omitempty"`
	ComposePath    *string  `json:"compose_path,omitempty"`
	Port           *int     `json:"port,omitempty"`
}

// UploadFileRequest represents a file upload for manual deployment
type UploadFileRequest struct {
	Path    string `json:"path"`
	Content []byte `json:"content"`
}

// FileInfo represents information about a file in an app's workspace
type FileInfo struct {
	Path    string    `json:"path"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
	IsDir   bool      `json:"is_dir"`
}
