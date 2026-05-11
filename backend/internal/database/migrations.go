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
		createAppBackupsTable,
		createAppRestoresTable,
		createIndexes,
	}

	for i, migration := range migrations {
		if _, err := db.Exec(migration); err != nil {
			return fmt.Errorf("migration %d failed: %w", i+1, err)
		}
	}

	// Run custom Go-based migration for domain -> domains
	if err := migrateDomainToDomainsFunc(db); err != nil {
		return fmt.Errorf("domain to domains migration failed: %w", err)
	}

	// Run S3 backup fields migration
	if err := migrateAppBackupsForS3(db); err != nil {
		return fmt.Errorf("S3 backup fields migration failed: %w", err)
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

const createAppBackupsTable = `
CREATE TABLE IF NOT EXISTS app_backups (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	backup_id TEXT NOT NULL UNIQUE,
	app_id INTEGER NOT NULL,
	status TEXT NOT NULL CHECK(status IN ('in_progress', 'success', 'failed')),
	path TEXT NOT NULL,
	size_bytes INTEGER NOT NULL DEFAULT 0,
	log TEXT,
	started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	completed_at DATETIME,
	FOREIGN KEY (app_id) REFERENCES apps(id) ON DELETE CASCADE
);
`

const createAppRestoresTable = `
CREATE TABLE IF NOT EXISTS app_restores (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	restore_id TEXT NOT NULL UNIQUE,
	app_id INTEGER NOT NULL,
	backup_id TEXT NOT NULL,
	status TEXT NOT NULL CHECK(status IN ('in_progress', 'success', 'failed')),
	log TEXT,
	started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	completed_at DATETIME,
	FOREIGN KEY (app_id) REFERENCES apps(id) ON DELETE CASCADE,
	FOREIGN KEY (backup_id) REFERENCES app_backups(backup_id) ON DELETE RESTRICT
);
`

const createIndexes = `
CREATE INDEX IF NOT EXISTS idx_apps_name ON apps(name);
CREATE INDEX IF NOT EXISTS idx_apps_status ON apps(status);
CREATE INDEX IF NOT EXISTS idx_deployment_logs_app_id ON deployment_logs(app_id);
CREATE INDEX IF NOT EXISTS idx_deployment_logs_started_at ON deployment_logs(started_at);
CREATE INDEX IF NOT EXISTS idx_app_backups_app_id ON app_backups(app_id);
CREATE INDEX IF NOT EXISTS idx_app_backups_started_at ON app_backups(started_at);
CREATE INDEX IF NOT EXISTS idx_app_restores_app_id ON app_restores(app_id);
CREATE INDEX IF NOT EXISTS idx_app_restores_started_at ON app_restores(started_at);
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

// migrateAppBackupsForS3 adds S3-related columns to app_backups table if they don't exist
func migrateAppBackupsForS3(db *sql.DB) error {
	// Check if the S3 columns already exist
	rows, err := db.Query("PRAGMA table_info(app_backups)")
	if err != nil {
		return fmt.Errorf("failed to query table info: %w", err)
	}
	defer rows.Close()

	s3PathExists := false
	uploadedToS3Exists := false
	s3UploadedAtExists := false

	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var dfltValue interface{}

		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			return fmt.Errorf("failed to scan column info: %w", err)
		}

		if name == "s3_path" {
			s3PathExists = true
		}
		if name == "uploaded_to_s3" {
			uploadedToS3Exists = true
		}
		if name == "s3_uploaded_at" {
			s3UploadedAtExists = true
		}
	}

	// Add missing columns
	if !s3PathExists {
		if _, err := db.Exec("ALTER TABLE app_backups ADD COLUMN s3_path TEXT DEFAULT ''"); err != nil {
			return fmt.Errorf("failed to add s3_path column: %w", err)
		}
	}

	if !uploadedToS3Exists {
		if _, err := db.Exec("ALTER TABLE app_backups ADD COLUMN uploaded_to_s3 BOOLEAN DEFAULT 0"); err != nil {
			return fmt.Errorf("failed to add uploaded_to_s3 column: %w", err)
		}
	}

	if !s3UploadedAtExists {
		if _, err := db.Exec("ALTER TABLE app_backups ADD COLUMN s3_uploaded_at DATETIME"); err != nil {
			return fmt.Errorf("failed to add s3_uploaded_at column: %w", err)
		}
	}

	return nil
}

