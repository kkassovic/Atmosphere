package services

import (
	"atmosphere/internal/config"
	"atmosphere/internal/models"
	"atmosphere/internal/repository"
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// BackupScheduler runs recurring app backups from database-backed schedules.
type BackupScheduler struct {
	repo      *repository.AppRepository
	appSvc    *AppService
	cfg       *config.Config
	interval  time.Duration
	stopOnce  sync.Once
	stopCh    chan struct{}
}

// NewBackupScheduler creates a scheduler that polls schedules on a fixed interval.
func NewBackupScheduler(repo *repository.AppRepository, appSvc *AppService, cfg *config.Config) *BackupScheduler {
	return &BackupScheduler{
		repo:     repo,
		appSvc:   appSvc,
		cfg:      cfg,
		interval: time.Minute,
		stopCh:   make(chan struct{}),
	}
}

// Run starts the scheduler loop.
func (s *BackupScheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.logf("backup scheduler started")
	s.runOnce(ctx)

	for {
		select {
		case <-ctx.Done():
			s.logf("backup scheduler stopped")
			return
		case <-s.stopCh:
			s.logf("backup scheduler stopped")
			return
		case <-ticker.C:
			s.runOnce(ctx)
		}
	}
}

// Stop requests scheduler shutdown.
func (s *BackupScheduler) Stop() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
}

func (s *BackupScheduler) runOnce(ctx context.Context) {
	now := time.Now().UTC()
	schedules, err := s.repo.ListDueAppBackupSchedules(now, 50)
	if err != nil {
		s.logf("failed to list due schedules: %v", err)
		return
	}

	for _, schedule := range schedules {
		s.runSchedule(ctx, schedule, now)
	}
}

func (s *BackupScheduler) runSchedule(ctx context.Context, schedule *models.AppBackupSchedule, now time.Time) {
	app, err := s.appSvc.repo.GetByID(schedule.AppID)
	if err != nil {
		s.markScheduleFailure(schedule, now, fmt.Errorf("failed to load app: %w", err))
		return
	}
	if app == nil {
		s.markScheduleFailure(schedule, now, fmt.Errorf("app not found"))
		return
	}

	backup, err := s.appSvc.CreateAppBackup(app.Name, schedule.UploadToS3)
	if err != nil {
		s.markScheduleFailure(schedule, now, err)
		return
	}

	lastRunAt := now
	nextRunAt := now.Add(time.Duration(schedule.IntervalMinutes) * time.Minute)
	schedule.LastBackupID = backup.BackupID
	schedule.LastRunAt = &lastRunAt
	schedule.NextRunAt = &nextRunAt
	schedule.LastStatus = "queued"
	schedule.LastError = ""
	if err := s.repo.UpsertAppBackupSchedule(schedule); err != nil {
		s.logf("failed to update schedule %d after backup start: %v", schedule.ID, err)
	}
	_ = ctx
	s.logf("started scheduled backup for app %s (%s)", app.Name, backup.BackupID)
}

func (s *BackupScheduler) markScheduleFailure(schedule *models.AppBackupSchedule, now time.Time, runErr error) {
	lastRunAt := now
	nextRunAt := now.Add(time.Duration(schedule.IntervalMinutes) * time.Minute)
	schedule.LastRunAt = &lastRunAt
	schedule.NextRunAt = &nextRunAt
	schedule.LastStatus = "failed"
	schedule.LastError = runErr.Error()
	if err := s.repo.UpsertAppBackupSchedule(schedule); err != nil {
		s.logf("failed to persist schedule failure for app_id=%d: %v", schedule.AppID, err)
	}
	s.logf("scheduled backup failed for app_id=%d: %v", schedule.AppID, runErr)
}

func (s *BackupScheduler) logf(format string, args ...interface{}) {
	log.Printf("[backup-scheduler] "+format, args...)
}