package database

import (
	"database/sql"
	"fmt"
)

// RunMigrations runs all database migrations
func RunMigrations(db *sql.DB) error {
	migrations := []string{
		createAppsTable,
		createDeploymentLogsTable,
		createIndexes,
	}

	for i, migration := range migrations {
		if _, err := db.Exec(migration); err != nil {
			return fmt.Errorf("migration %d failed: %w", i+1, err)
		}
	}

	return nil
}

const createAppsTable = `
CREATE TABLE IF NOT EXISTS apps (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL UNIQUE,
	deployment_type TEXT NOT NULL CHECK(deployment_type IN ('github', 'manual')),
	build_type TEXT NOT NULL CHECK(build_type IN ('dockerfile', 'compose')),
	status TEXT NOT NULL DEFAULT 'stopped' CHECK(status IN ('stopped', 'running', 'building', 'failed')),
	domain TEXT,
	env_vars TEXT DEFAULT '{}',
	github_repo TEXT,
	github_branch TEXT,
	github_subdir TEXT,
	dockerfile_path TEXT,
	compose_path TEXT,
	port INTEGER,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	last_deployed_at DATETIME
);
`

const createDeploymentLogsTable = `
CREATE TABLE IF NOT EXISTS deployment_logs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	app_id INTEGER NOT NULL,
	status TEXT NOT NULL CHECK(status IN ('success', 'failed', 'in_progress')),
	log TEXT,
	started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	ended_at DATETIME,
	FOREIGN KEY (app_id) REFERENCES apps(id) ON DELETE CASCADE
);
`

const createIndexes = `
CREATE INDEX IF NOT EXISTS idx_apps_name ON apps(name);
CREATE INDEX IF NOT EXISTS idx_apps_status ON apps(status);
CREATE INDEX IF NOT EXISTS idx_deployment_logs_app_id ON deployment_logs(app_id);
CREATE INDEX IF NOT EXISTS idx_deployment_logs_started_at ON deployment_logs(started_at);
`
