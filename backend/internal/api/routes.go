package api

import (
	"atmosphere/internal/config"
	"atmosphere/internal/repository"
	"atmosphere/internal/services"
	"database/sql"
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
	appService := services.NewAppService(appRepo, cfg, deploymentService)

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

		// File upload (manual deployments)
		r.Post("/apps/{name}/files", handler.UploadFile)
	})

	return r
}
