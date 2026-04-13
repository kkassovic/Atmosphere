package api

import (
	"atmosphere/internal/models"
	"atmosphere/internal/services"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// Handler handles HTTP requests
type Handler struct {
	appService *services.AppService
}

// NewHandler creates a new handler
func NewHandler(appService *services.AppService) *Handler {
	return &Handler{
		appService: appService,
	}
}

// CreateApp handles POST /api/v1/apps
func (h *Handler) CreateApp(w http.ResponseWriter, r *http.Request) {
	var req models.CreateAppRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fmt.Printf("Error decoding request body: %v\n", err)
		respondError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}

	app, err := h.appService.CreateApp(&req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, app)
}

// GetApp handles GET /api/v1/apps/{name}
func (h *Handler) GetApp(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	app, err := h.appService.GetApp(name)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, app)
}

// ListApps handles GET /api/v1/apps
func (h *Handler) ListApps(w http.ResponseWriter, r *http.Request) {
	apps, err := h.appService.ListApps()
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, apps)
}

// UpdateApp handles PUT /api/v1/apps/{name}
func (h *Handler) UpdateApp(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	var req models.UpdateAppRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	app, err := h.appService.UpdateApp(name, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, app)
}

// DeleteApp handles DELETE /api/v1/apps/{name}
func (h *Handler) DeleteApp(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	if err := h.appService.DeleteApp(name); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "App deleted successfully"})
}

// DeployApp handles POST /api/v1/apps/{name}/deploy
func (h *Handler) DeployApp(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	deployLog, err := h.appService.DeployApp(name)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusAccepted, map[string]interface{}{
		"message":        "Deployment started",
		"deployment_log": deployLog,
	})
}

// StartApp handles POST /api/v1/apps/{name}/start
func (h *Handler) StartApp(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	if err := h.appService.StartApp(name); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "App started successfully"})
}

// StopApp handles POST /api/v1/apps/{name}/stop
func (h *Handler) StopApp(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	if err := h.appService.StopApp(name); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "App stopped successfully"})
}

// GetDeploymentLogs handles GET /api/v1/apps/{name}/logs
func (h *Handler) GetDeploymentLogs(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	// Parse limit from query params
	limitStr := r.URL.Query().Get("limit")
	limit := 10
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	logs, err := h.appService.GetDeploymentLogs(name, limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, logs)
}

// UploadFile handles POST /api/v1/apps/{name}/files
func (h *Handler) UploadFile(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	// Parse multipart form
	if err := r.ParseMultipartForm(10 << 20); err != nil { // 10 MB max
		respondError(w, http.StatusBadRequest, "Failed to parse form")
		return
	}

	// Get file path
	filePath := r.FormValue("path")
	if filePath == "" {
		respondError(w, http.StatusBadRequest, "path is required")
		return
	}

	// Get file content
	file, _, err := r.FormFile("content")
	if err != nil {
		// Try to get content from form value instead
		contentStr := r.FormValue("content")
		if contentStr == "" {
			respondError(w, http.StatusBadRequest, "content is required")
			return
		}
		
		// Upload from string content
		if err := h.appService.UploadFile(name, filePath, []byte(contentStr)); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
	} else {
		defer file.Close()

		// Read file content
		content, err := io.ReadAll(file)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to read file")
			return
		}

		// Upload file
		if err := h.appService.UploadFile(name, filePath, content); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": fmt.Sprintf("File %s uploaded successfully", filePath),
	})
}

// ListFiles handles GET /api/v1/apps/{name}/files
func (h *Handler) ListFiles(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	files, err := h.appService.ListFiles(name)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, files)
}

// GetFile handles GET /api/v1/apps/{name}/files/{filePath}
func (h *Handler) GetFile(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	filePath := chi.URLParam(r, "*") // Capture everything after /files/

	if filePath == "" {
		respondError(w, http.StatusBadRequest, "file path is required")
		return
	}

	content, err := h.appService.GetFile(name, filePath)
	if err != nil {
		if err.Error() == "file not found" {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Return the file content as plain text
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write(content)
}

// GetMergedComposeConfig handles GET /api/v1/apps/{name}/compose-config
func (h *Handler) GetMergedComposeConfig(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	config, err := h.appService.GetMergedComposeConfig(name)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Return as plain text YAML
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(config))
}

// Helper functions for responses

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}
