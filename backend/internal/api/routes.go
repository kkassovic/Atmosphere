package api

import (
	"atmosphere/internal/config"
	"atmosphere/internal/repository"
	"atmosphere/internal/services"
	"atmosphere/internal/storage"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// NewRouter creates and configures the HTTP router
func NewRouter(db *sql.DB, cfg *config.Config) http.Handler {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// CORS
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	// Initialize services
	appRepo := repository.NewAppRepository(db)
	dockerService, err := services.NewDockerService()
	if err != nil {
		panic(err) // Fatal error if Docker is not available
	}
	deploymentService := services.NewDeploymentService(cfg, dockerService)

	// Initialize backup storage
	storageConfig := &storage.StorageConfig{
		LocalBasePath: cfg.LogsDir,
	}
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
		fmt.Printf("Warning: failed to initialize backup storage: %v\n", err)
		// Fall back to local storage
		backupStorage, _ = storage.NewLocalStorage(cfg.LogsDir)
	}

	appService := services.NewAppService(appRepo, cfg, deploymentService, backupStorage)

	// Initialize handler
	handler := NewHandler(appService)

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// API routes
	r.Route("/api/v1", func(r chi.Router) {
		// Apps
		r.Get("/apps", handler.ListApps)
		r.Post("/apps", handler.CreateApp)
		r.Get("/apps/{name}", handler.GetApp)
		r.Put("/apps/{name}", handler.UpdateApp)
		r.Delete("/apps/{name}", handler.DeleteApp)

		// Deployment actions
		r.Post("/apps/{name}/deploy", handler.DeployApp)
		r.Post("/apps/{name}/start", handler.StartApp)
		r.Post("/apps/{name}/stop", handler.StopApp)

		// Logs
		r.Get("/apps/{name}/logs", handler.GetDeploymentLogs)

		// App backups and restores
		r.Post("/apps/{name}/backups", handler.CreateAppBackup)
		r.Get("/apps/{name}/backups", handler.ListAppBackups)
		r.Get("/apps/{name}/backups/{backupID}", handler.GetAppBackup)
		r.Delete("/apps/{name}/backups/{backupID}", handler.DeleteAppBackup)
		r.Post("/apps/{name}/restores", handler.StartAppRestore)
		r.Get("/apps/{name}/restores/{restoreID}", handler.GetAppRestore)

		// File upload (manual deployments)
		r.Post("/apps/{name}/files", handler.UploadFile)
		
		// File operations
		r.Get("/apps/{name}/files", handler.ListFiles)
		r.Get("/apps/{name}/files/*", handler.GetFile)
		
		// Compose config
		r.Get("/apps/{name}/compose-config", handler.GetMergedComposeConfig)

		// Backup storage health check
		r.Get("/backup-storage/health", handler.CheckBackupStorageHealth)
	})

	return r
}
