package main

import (
	"atmosphere/internal/api"
	"atmosphere/internal/config"
	"atmosphere/internal/database"
	"atmosphere/internal/repository"
	"atmosphere/internal/services"
	"atmosphere/internal/storage"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize database
	db, err := database.InitDB(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Run migrations
	if err := database.RunMigrations(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Create required directories
	if err := createDirectories(cfg); err != nil {
		log.Fatalf("Failed to create directories: %v", err)
	}

	appRepo := repository.NewAppRepository(db)
	dockerService, err := services.NewDockerService()
	if err != nil {
		log.Fatalf("Failed to initialize Docker service: %v", err)
	}
	deploymentService := services.NewDeploymentService(cfg, dockerService)

	storageConfig := &storage.StorageConfig{LocalBasePath: cfg.LogsDir}
	if cfg.IsS3Enabled() {
		storageConfig.Type = "s3"
		storageConfig.S3Endpoint = cfg.S3Endpoint
		storageConfig.S3Bucket = cfg.S3Bucket
		storageConfig.S3Region = cfg.S3Region
		storageConfig.S3AccessKey = cfg.S3AccessKey
		storageConfig.S3SecretKey = cfg.S3SecretKey
		storageConfig.S3PathPrefix = cfg.S3PathPrefix
	}
	backupStorage, err := storage.NewBackupStorage(storageConfig)
	if err != nil {
		log.Printf("Warning: failed to initialize backup storage: %v", err)
		backupStorage, _ = storage.NewLocalStorage(cfg.LogsDir)
	}

	appService := services.NewAppService(appRepo, cfg, deploymentService, backupStorage)
	backupScheduler := services.NewBackupScheduler(appRepo, appService, cfg)

	// Initialize router
	router := api.NewRouter(appService)

	// Create HTTP server
	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Starting Atmosphere on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Start backup scheduler in background.
	schedulerCtx, schedulerCancel := context.WithCancel(context.Background())
	go backupScheduler.Run(schedulerCtx)

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	schedulerCancel()
	backupScheduler.Stop()

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped")
}

// createDirectories ensures all required directories exist
func createDirectories(cfg *config.Config) error {
	dirs := []string{
		cfg.WorkspacesDir,
		cfg.KeysDir,
		cfg.LogsDir,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Keys directory needs restrictive permissions
	if err := os.Chmod(cfg.KeysDir, 0700); err != nil {
		return fmt.Errorf("failed to set permissions on keys directory: %w", err)
	}

	return nil
}
