package api

import (
	"atmosphere/internal/services"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// NewRouter creates and configures the HTTP router
func NewRouter(appService *services.AppService) http.Handler {
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

	if appService == nil {
		panic("app service is required")
	}

	// Initialize handler
	handler := NewHandler(appService)

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// API routes
	r.Route("/api/v1", func(r chi.Router) {
		// Templates
		r.Get("/templates", handler.ListTemplates)
		r.Get("/templates/{id}", handler.GetTemplate)
		r.Post("/templates/{id}/provision", handler.ProvisionTemplate)

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
		r.Get("/apps/{name}/backup-schedule", handler.GetAppBackupSchedule)
		r.Put("/apps/{name}/backup-schedule", handler.UpsertAppBackupSchedule)
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
