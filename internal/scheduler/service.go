package scheduler

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/neoclaw-ai/neoclaw/internal/logging"
	"github.com/robfig/cron/v3"
)

// Service runs scheduled jobs backed by one jobs.json file.
type Service struct {
	store   *jobStore
	runner  *Runner
	cron    *cron.Cron
	started bool
	runCtx  context.Context
	mu      sync.Mutex
}

// NewService creates a direct cron-backed scheduler service over one jobs.json path.
func NewService(path string, runner *Runner) *Service {
	return &Service{
		store:  newJobStore(path),
		runner: runner,
		cron: cron.New(
			cron.WithLocation(time.Local),
			cron.WithChain(cron.SkipIfStillRunning(cron.DefaultLogger)),
		),
	}
}

// Start loads enabled jobs from the store and starts cron execution.
func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return errors.New("scheduler already started")
	}

	jobs, err := s.store.List(ctx)
	if err != nil {
		return err
	}

	s.store.clearEntryIDs()
	for _, job := range jobs {
		if !job.Enabled {
			continue
		}
		if err := s.addCronEntry(job, ctx); err != nil {
			return fmt.Errorf("register cron job %s: %w", job.ID, err)
		}
	}

	s.cron.Start()
	s.started = true
	s.runCtx = ctx
	logging.Logger().Info("scheduler started", "jobs_registered", len(jobs))
	return nil
}

// Stop stops cron and waits for in-flight callbacks to finish or ctx cancellation.
func (s *Service) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started {
		return nil
	}

	doneCtx := s.cron.Stop()
	s.started = false
	s.runCtx = nil
	s.store.clearEntryIDs()
	select {
	case <-doneCtx.Done():
		logging.Logger().Info("scheduler stopped")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// RunNow executes one job immediately by ID.
func (s *Service) RunNow(ctx context.Context, jobID string) (string, error) {
	job, err := s.store.get(ctx, jobID)
	if err != nil {
		return "", err
	}

	logging.Logger().Info(
		"job run",
		"job_id", job.ID,
		"source", "manual",
		"action", job.Action,
		"channel_id", job.ChannelID,
	)

	output, err := s.runner.Run(ctx, job)
	if err != nil {
		logging.Logger().Warn(
			"job run failed",
			"job_id", job.ID,
			"source", "manual",
			"action", job.Action,
			"err", err,
		)
		return "", err
	}

	logging.Logger().Info(
		"job run complete",
		"job_id", job.ID,
		"source", "manual",
		"action", job.Action,
	)
	return output, nil
}

// List returns all persisted scheduled jobs.
func (s *Service) List(ctx context.Context) ([]Job, error) {
	return s.store.List(ctx)
}

// Create persists a scheduled job and registers it in running cron when active.
func (s *Service) Create(ctx context.Context, in CreateInput) (Job, error) {
	job, err := s.store.Create(ctx, in)
	if err != nil {
		return Job{}, err
	}
	if err := s.register(ctx, job); err != nil {
		return Job{}, err
	}
	return job, nil
}

// Delete removes a scheduled job and unregisters it from running cron.
func (s *Service) Delete(ctx context.Context, jobID string) error {
	if err := s.store.Delete(ctx, jobID); err != nil {
		return err
	}
	s.unregister(jobID)
	return nil
}

func (s *Service) register(ctx context.Context, job Job) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started || !job.Enabled {
		return nil
	}

	if err := s.addCronEntry(job, s.runCtx); err != nil {
		return fmt.Errorf("register cron job %s: %w", job.ID, err)
	}
	return nil
}

func (s *Service) unregister(jobID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started {
		s.store.deleteEntryID(jobID)
		return
	}

	entryID, ok := s.store.entryID(jobID)
	if !ok {
		return
	}
	s.cron.Remove(entryID)
	s.store.deleteEntryID(jobID)
}

func (s *Service) addCronEntry(job Job, runCtx context.Context) error {
	capturedJob := job
	entryID, err := s.cron.AddFunc(capturedJob.Cron, func() {
		output, runErr := s.runner.Run(runCtx, capturedJob)
		if runErr != nil {
			logging.Logger().Warn(
				"scheduled job failed",
				"job_id", capturedJob.ID,
				"action", capturedJob.Action,
				"err", runErr,
			)
			return
		}
		logging.Logger().Info(
			"scheduled job succeeded",
			"job_id", capturedJob.ID,
			"action", capturedJob.Action,
			"output_len", len(output),
		)
	})
	if err != nil {
		return err
	}
	s.store.setEntryID(capturedJob.ID, entryID)
	return nil
}
