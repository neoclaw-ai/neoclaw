// Package scheduler provides persistent job storage and runtime scheduling.
package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/machinae/betterclaw/internal/store"
	"github.com/robfig/cron/v3"
)

// Action identifies which deterministic operation a scheduled job executes.
type Action string

const (
	// ActionSendMessage sends a message through the active channel sender.
	ActionSendMessage Action = "send_message"
	// ActionRunCommand executes a shell command.
	ActionRunCommand Action = "run_command"
	// ActionHTTPRequest performs an HTTP request.
	ActionHTTPRequest Action = "http_request"
)

// Job is one persisted scheduled task in jobs.json.
type Job struct {
	ID          string         `json:"id"`
	Description string         `json:"description"`
	Cron        string         `json:"cron"`
	Action      Action         `json:"action"`
	Args        map[string]any `json:"args"`
	ChannelID   string         `json:"channel_id"`
	Enabled     bool           `json:"enabled"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// CreateInput contains fields required to create a job.
type CreateInput struct {
	Description string
	Cron        string
	Action      Action
	Args        map[string]any
	ChannelID   string
}

// Store manages CRUD operations for jobs persisted at one jobs.json path.
type Store struct {
	path string
	mu   sync.Mutex
}

// NewStore constructs a jobs store over one jobs.json file path.
func NewStore(path string) *Store {
	return &Store{path: path}
}

// Path returns the configured jobs.json path.
func (s *Store) Path() string {
	return s.path
}

// List returns all jobs from jobs.json.
func (s *Store) List(ctx context.Context) ([]Job, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readLocked()
}

// Get returns one job by ID.
func (s *Store) Get(ctx context.Context, id string) (Job, error) {
	if err := ctx.Err(); err != nil {
		return Job{}, err
	}
	target := strings.TrimSpace(id)
	if target == "" {
		return Job{}, errors.New("job id is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	jobs, err := s.readLocked()
	if err != nil {
		return Job{}, err
	}
	for _, job := range jobs {
		if job.ID == target {
			return job, nil
		}
	}
	return Job{}, fmt.Errorf("job %q not found", target)
}

// Create validates and persists a new enabled job.
func (s *Store) Create(ctx context.Context, in CreateInput) (Job, error) {
	if err := ctx.Err(); err != nil {
		return Job{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	jobs, err := s.readLocked()
	if err != nil {
		return Job{}, err
	}

	now := time.Now().UTC()
	job := Job{
		ID:          newJobID(now),
		Description: strings.TrimSpace(in.Description),
		Cron:        strings.TrimSpace(in.Cron),
		Action:      in.Action,
		Args:        cloneArgs(in.Args),
		ChannelID:   strings.TrimSpace(in.ChannelID),
		Enabled:     true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := validateJob(job); err != nil {
		return Job{}, err
	}

	jobs = append(jobs, job)
	if err := s.writeLocked(jobs); err != nil {
		return Job{}, err
	}
	return job, nil
}

// Delete removes one job by ID.
func (s *Store) Delete(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	target := strings.TrimSpace(id)
	if target == "" {
		return errors.New("job id is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	jobs, err := s.readLocked()
	if err != nil {
		return err
	}
	for i := range jobs {
		if jobs[i].ID != target {
			continue
		}
		jobs = append(jobs[:i], jobs[i+1:]...)
		return s.writeLocked(jobs)
	}
	return fmt.Errorf("job %q not found", target)
}

func (s *Store) readLocked() ([]Job, error) {
	if strings.TrimSpace(s.path) == "" {
		return nil, errors.New("jobs store path is required")
	}

	content, err := store.ReadFile(s.path)
	switch {
	case err == nil:
	case errors.Is(err, os.ErrNotExist):
		return []Job{}, nil
	default:
		return nil, fmt.Errorf("read jobs file %q: %w", s.path, err)
	}

	if len(strings.TrimSpace(content)) == 0 {
		return []Job{}, nil
	}

	var jobs []Job
	if err := json.Unmarshal([]byte(content), &jobs); err != nil {
		return nil, fmt.Errorf("decode jobs file %q: %w", s.path, err)
	}
	for _, job := range jobs {
		if err := validateJob(job); err != nil {
			return nil, fmt.Errorf("invalid job %q: %w", job.ID, err)
		}
	}
	return jobs, nil
}

func (s *Store) writeLocked(jobs []Job) error {
	if strings.TrimSpace(s.path) == "" {
		return errors.New("jobs store path is required")
	}

	encoded, err := json.MarshalIndent(jobs, "", "  ")
	if err != nil {
		return fmt.Errorf("encode jobs: %w", err)
	}
	encoded = append(encoded, '\n')

	if err := store.WriteFile(s.path, encoded); err != nil {
		return fmt.Errorf("replace jobs file: %w", err)
	}
	return nil
}

func validateJob(job Job) error {
	if strings.TrimSpace(job.ID) == "" {
		return errors.New("job id is required")
	}
	if strings.TrimSpace(job.Description) == "" {
		return errors.New("job description is required")
	}
	if strings.TrimSpace(job.ChannelID) == "" {
		return errors.New("job channel_id is required")
	}
	if err := validateAction(job.Action); err != nil {
		return err
	}
	if err := validateCron(job.Cron); err != nil {
		return err
	}
	if job.Args == nil {
		return errors.New("job args are required")
	}
	return nil
}

func validateAction(action Action) error {
	switch action {
	case ActionSendMessage, ActionRunCommand, ActionHTTPRequest:
		return nil
	default:
		return fmt.Errorf("unsupported job action %q", action)
	}
}

func validateCron(spec string) error {
	trimmed := strings.TrimSpace(spec)
	if trimmed == "" {
		return errors.New("job cron is required")
	}
	if _, err := cron.ParseStandard(trimmed); err != nil {
		return fmt.Errorf("invalid cron expression %q: %w", spec, err)
	}
	return nil
}

func newJobID(now time.Time) string {
	return fmt.Sprintf("job_%d", now.UnixNano())
}

func cloneArgs(src map[string]any) map[string]any {
	if src == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(src))
	for key, value := range src {
		out[key] = value
	}
	return out
}
