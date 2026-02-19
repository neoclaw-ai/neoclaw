package scheduler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/machinae/betterclaw/internal/logging"
	"github.com/robfig/cron/v3"
)

// Service runs scheduled jobs from a Store.
type Service struct {
	store   *Store
	runner  *Runner
	cron    *cron.Cron
	started bool
}

// NewService creates a direct cron-backed scheduler service.
func NewService(store *Store, runner *Runner) *Service {
	return &Service{
		store:  store,
		runner: runner,
		cron: cron.New(
			cron.WithLocation(time.Local),
			cron.WithChain(cron.SkipIfStillRunning(cron.DefaultLogger)),
		),
	}
}

// Start loads enabled jobs from the store and starts cron execution.
func (s *Service) Start(ctx context.Context) error {
	if s.started {
		return errors.New("scheduler already started")
	}

	jobs, err := s.store.List(ctx)
	if err != nil {
		return err
	}

	for _, job := range jobs {
		if !job.Enabled {
			continue
		}
		job := job
		_, err := s.cron.AddFunc(job.Cron, func() {
			output, runErr := s.runner.Run(ctx, job)
			if runErr != nil {
				logging.Logger().Warn(
					"scheduled job failed",
					"job_id", job.ID,
					"action", job.Action,
					"err", runErr,
				)
				return
			}
			logging.Logger().Info(
				"scheduled job succeeded",
				"job_id", job.ID,
				"action", job.Action,
				"output_len", len(output),
			)
		})
		if err != nil {
			return fmt.Errorf("register cron job %q: %w", job.ID, err)
		}
	}

	s.cron.Start()
	s.started = true
	logging.Logger().Info("scheduler started", "jobs_registered", len(jobs))
	return nil
}

// Stop stops cron and waits for in-flight callbacks to finish or ctx cancellation.
func (s *Service) Stop(ctx context.Context) error {
	if !s.started {
		return nil
	}

	doneCtx := s.cron.Stop()
	s.started = false
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
	job, err := s.store.Get(ctx, jobID)
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
