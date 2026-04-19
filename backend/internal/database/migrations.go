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
		if err := db.Exec(migration); err != nil {
			return fmt.Errorf("migration %d failed: %w", i+1, err)
		}
	}

	// Run custom Go-based migration for domain -> domains
	if err := migrateDomainToDomainsFunc(db); err != nil {
		return fmt.Errorf("domain to domains migration failed: %w", err)
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
	domains TEXT DEFAULT '[]',
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

// migrateDomainToDomainsFunc handles the migration from domain (string) to domains (JSON array)
// This is safe to run multiple times - it checks column existence first
func migrateDomainToDomainsFunc(db *sql.DB) error {
	// Check if domains column exists
	var domainsExists bool
	rows, err := db.Query("PRAGMA table_info(apps)")
	if err != nil {
		return fmt.Errorf("failed to get table info: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var typ string
		var notnull int
		var dfltValue interface{}
		var pk int
		
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			return fmt.Errorf("failed to scan column info: %w", err)
		}
		
		if name == "domains" {
			domainsExists = true
			break
		}
	}

	// If domains column doesn't exist, add it and migrate data
	if !domainsExists {
		// Add domains column
		if _, err := db.Exec("ALTER TABLE apps ADD COLUMN domains TEXT DEFAULT '[]'"); err != nil {
			return fmt.Errorf("failed to add domains column: %w", err)
		}

		// Migrate data from domain to domains
		_, err := db.Exec(`
			UPDATE apps 
			SET domains = CASE 
				WHEN domain IS NOT NULL AND domain != '' 
				THEN json_array(domain) 
				ELSE '[]' 
			END
		`)
		if err != nil {
			return fmt.Errorf("failed to migrate domain data: %w", err)
		}
	}

	return nil
}

