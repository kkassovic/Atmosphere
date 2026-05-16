package services

import (
	"atmosphere/internal/config"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ResetService performs a hard reset of all Atmosphere-managed data.
type ResetService struct {
	dockerService *DockerService
	cfg           *config.Config
}

// HardResetResult describes what was removed during the reset.
type HardResetResult struct {
	ContainersRemoved int      `json:"containers_removed"`
	VolumesRemoved    int      `json:"volumes_removed"`
	DirsWiped         []string `json:"dirs_wiped"`
	DatabaseDeleted   bool     `json:"database_deleted"`
	IniFilesPreserved int      `json:"ini_files_preserved"`
	Errors            []string `json:"errors,omitempty"`
}

// NewResetService creates a new ResetService.
func NewResetService(cfg *config.Config, dockerService *DockerService) *ResetService {
	return &ResetService{cfg: cfg, dockerService: dockerService}
}

// HardReset wipes all Atmosphere data:
//   - stops and removes all atmosphere containers
//   - removes all atmosphere Docker volumes
//   - wipes WorkspacesDir, KeysDir, LogsDir (preserving *.ini files)
//   - deletes the SQLite database
//
// S3 backups are never touched.
func (s *ResetService) HardReset(ctx context.Context) (*HardResetResult, error) {
	result := &HardResetResult{}

	// 1. Stop and remove all atmosphere containers.
	containersRemoved, errs := s.removeAllContainers(ctx)
	result.ContainersRemoved = containersRemoved
	result.Errors = append(result.Errors, errs...)

	// 2. Remove all atmosphere Docker volumes.
	volumesRemoved, errs := s.removeAllVolumes(ctx)
	result.VolumesRemoved = volumesRemoved
	result.Errors = append(result.Errors, errs...)

	// 3. Wipe managed directories, preserving *.ini files.
	dirs := []string{s.cfg.WorkspacesDir, s.cfg.KeysDir, s.cfg.LogsDir}
	for _, dir := range dirs {
		preserved, err := wipeDir(dir)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("wipe %s: %v", dir, err))
			continue
		}
		result.IniFilesPreserved += preserved
		result.DirsWiped = append(result.DirsWiped, dir)
	}

	// 4. Delete the SQLite database.
	if err := os.Remove(s.cfg.DatabasePath); err != nil && !os.IsNotExist(err) {
		result.Errors = append(result.Errors, fmt.Sprintf("delete database: %v", err))
	} else {
		result.DatabaseDeleted = true
	}

	return result, nil
}

// removeAllContainers stops and removes every container bearing the atmosphere.app label.
func (s *ResetService) removeAllContainers(ctx context.Context) (int, []string) {
	containers, err := s.dockerService.ListAllAtmosphereContainers(ctx)
	if err != nil {
		return 0, []string{fmt.Sprintf("list containers: %v", err)}
	}

	var errs []string
	removed := 0
	for _, c := range containers {
		if err := s.dockerService.RemoveContainer(ctx, c.ID); err != nil {
			errs = append(errs, fmt.Sprintf("remove container %s: %v", c.ID[:12], err))
			continue
		}
		removed++
	}
	return removed, errs
}

// removeAllVolumes removes every named Docker volume associated with atmosphere compose projects.
func (s *ResetService) removeAllVolumes(ctx context.Context) (int, []string) {
	names, err := s.dockerService.ListAllAtmosphereVolumes(ctx)
	if err != nil {
		return 0, []string{fmt.Sprintf("list volumes: %v", err)}
	}

	var errs []string
	removed := 0
	for _, name := range names {
		if err := s.dockerService.RemoveVolume(ctx, name); err != nil {
			errs = append(errs, fmt.Sprintf("remove volume %s: %v", name, err))
			continue
		}
		removed++
	}
	return removed, errs
}

// wipeDir removes all contents of dir except *.ini files.
// Returns the count of *.ini files that were preserved.
func wipeDir(dir string) (int, error) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return 0, nil
	}

	// Collect *.ini files: relative path → contents.
	iniFiles := map[string][]byte{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if !d.IsDir() && strings.ToLower(filepath.Ext(path)) == ".ini" {
			data, readErr := os.ReadFile(path)
			if readErr == nil {
				rel, _ := filepath.Rel(dir, path)
				iniFiles[rel] = data
			}
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("walk dir: %w", err)
	}

	// Remove all directory entries.
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, fmt.Errorf("read dir: %w", err)
	}
	for _, entry := range entries {
		if removeErr := os.RemoveAll(filepath.Join(dir, entry.Name())); removeErr != nil {
			return 0, fmt.Errorf("remove %s: %w", entry.Name(), removeErr)
		}
	}

	// Restore *.ini files.
	for rel, data := range iniFiles {
		dest := filepath.Join(dir, rel)
		if mkdirErr := os.MkdirAll(filepath.Dir(dest), 0755); mkdirErr != nil {
			continue
		}
		_ = os.WriteFile(dest, data, 0644)
	}

	return len(iniFiles), nil
}
