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

// CreateAppBackup handles POST /api/v1/apps/{name}/backups
func (h *Handler) CreateAppBackup(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	var req models.CreateAppBackupRequest
	// Decode request body if present (optional)
	_ = json.NewDecoder(r.Body).Decode(&req)

	backup, err := h.appService.CreateAppBackup(name, req.UploadToS3)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusAccepted, map[string]interface{}{
		"message": "App backup started",
		"backup":  backup,
	})
}

// ListAppBackups handles GET /api/v1/apps/{name}/backups
func (h *Handler) ListAppBackups(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	limit := 20
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	backups, err := h.appService.ListAppBackups(name, limit)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, backups)
}

// GetAppBackup handles GET /api/v1/apps/{name}/backups/{backupID}
func (h *Handler) GetAppBackup(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	backupID := chi.URLParam(r, "backupID")

	backup, err := h.appService.GetAppBackup(name, backupID)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, backup)
}

// DeleteAppBackup handles DELETE /api/v1/apps/{name}/backups/{backupID}
func (h *Handler) DeleteAppBackup(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	backupID := chi.URLParam(r, "backupID")

	if err := h.appService.DeleteAppBackup(name, backupID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Backup deleted successfully"})
}

// StartAppRestore handles POST /api/v1/apps/{name}/restores
func (h *Handler) StartAppRestore(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	var req models.CreateAppRestoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.BackupID == "" {
		respondError(w, http.StatusBadRequest, "backup_id is required")
		return
	}

	restore, err := h.appService.StartAppRestore(name, req.BackupID, req.SourceApp, req.RestoreAsNew, req.NewAppName)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusAccepted, map[string]interface{}{
		"message": "App restore started",
		"restore": restore,
	})
}

// GetAppRestore handles GET /api/v1/apps/{name}/restores/{restoreID}
func (h *Handler) GetAppRestore(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	restoreID := chi.URLParam(r, "restoreID")

	restore, err := h.appService.GetAppRestore(name, restoreID)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, restore)
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

// CheckBackupStorageHealth handles GET /api/v1/backup-storage/health
func (h *Handler) CheckBackupStorageHealth(w http.ResponseWriter, r *http.Request) {
	health, err := h.appService.CheckBackupStorageHealth(r.Context())
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"status": "error",
			"error":  err.Error(),
		})
		return
	}

	respondJSON(w, http.StatusOK, health)
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
