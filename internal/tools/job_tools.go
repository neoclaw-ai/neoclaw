package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/neoclaw-ai/neoclaw/internal/scheduler"
)

// JobListTool lists scheduled jobs from jobs.json.
type JobListTool struct {
	Service *scheduler.Service
}

// Name returns the tool name.
func (t JobListTool) Name() string {
	return "job_list"
}

// Description returns the tool description for the model.
func (t JobListTool) Description() string {
	return "List all scheduled jobs"
}

// Schema returns the JSON schema for job_list args.
func (t JobListTool) Schema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

// Permission declares default permission behavior for this tool.
func (t JobListTool) Permission() Permission {
	return AutoApprove
}

// Execute lists all scheduled jobs.
func (t JobListTool) Execute(ctx context.Context, _ map[string]any) (*ToolResult, error) {
	if t.Service == nil {
		return nil, errors.New("job service is required")
	}
	jobs, err := t.Service.List(ctx)
	if err != nil {
		return nil, err
	}
	if len(jobs) == 0 {
		return &ToolResult{Output: "No scheduled jobs."}, nil
	}

	var b strings.Builder
	b.WriteString("Scheduled jobs:\n")
	for i, job := range jobs {
		status := "disabled"
		if job.Enabled {
			status = "enabled"
		}
		fmt.Fprintf(&b, "%d. %s (%s) - %s\n", i+1, job.Description, job.Cron, status)
		fmt.Fprintf(&b, "   id: %s, action: %s, channel: %s", job.ID, job.Action, job.ChannelID)
		if i < len(jobs)-1 {
			b.WriteByte('\n')
		}
	}
	return TruncateOutput(b.String())
}

// JobCreateTool creates a scheduled job.
type JobCreateTool struct {
	Service          *scheduler.Service
	ChannelID        string
	ResolveChannelID func() string
}

// Name returns the tool name.
func (t JobCreateTool) Name() string {
	return "job_create"
}

// Description returns the tool description for the model.
func (t JobCreateTool) Description() string {
	return "Create a new scheduled job"
}

// Schema returns the JSON schema for job_create args.
func (t JobCreateTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"description": map[string]any{
				"type":        "string",
				"description": "Human-readable description",
			},
			"cron": map[string]any{
				"type":        "string",
				"description": "Cron expression (server local timezone)",
			},
			"action": map[string]any{
				"type":        "string",
				"description": "One of: send_message, run_command, http_request",
			},
			"args": map[string]any{
				"type":        "object",
				"description": "Action-specific arguments",
			},
		},
		"required": []string{"description", "cron", "action", "args"},
	}
}

// Permission declares default permission behavior for this tool.
func (t JobCreateTool) Permission() Permission {
	return AutoApprove
}

// Execute validates and persists a new scheduled job.
func (t JobCreateTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	if t.Service == nil {
		return nil, errors.New("job service is required")
	}
	description, err := stringArg(args, "description")
	if err != nil {
		return nil, err
	}
	cronSpec, err := stringArg(args, "cron")
	if err != nil {
		return nil, err
	}
	actionRaw, err := stringArg(args, "action")
	if err != nil {
		return nil, err
	}
	action := scheduler.Action(strings.TrimSpace(actionRaw))
	if err := validateJobAction(action); err != nil {
		return nil, err
	}
	jobArgs, err := objectArg(args, "args")
	if err != nil {
		return nil, err
	}

	channelID := strings.TrimSpace(t.ChannelID)
	if t.ResolveChannelID != nil {
		if resolved := strings.TrimSpace(t.ResolveChannelID()); resolved != "" {
			channelID = resolved
		}
	}
	if channelID == "" {
		channelID = "cli"
	}
	createInput := scheduler.CreateInput{
		Description: description,
		Cron:        cronSpec,
		Action:      action,
		Args:        jobArgs,
		ChannelID:   channelID,
	}

	job, err := t.Service.Create(ctx, createInput)
	if err != nil {
		return nil, err
	}

	return &ToolResult{Output: fmt.Sprintf("created job %s", job.ID)}, nil
}

// JobDeleteTool deletes a scheduled job by ID.
type JobDeleteTool struct {
	Service *scheduler.Service
}

// Name returns the tool name.
func (t JobDeleteTool) Name() string {
	return "job_delete"
}

// Description returns the tool description for the model.
func (t JobDeleteTool) Description() string {
	return "Delete a scheduled job"
}

// Schema returns the JSON schema for job_delete args.
func (t JobDeleteTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id": map[string]any{
				"type":        "string",
				"description": "Scheduled job ID",
			},
		},
		"required": []string{"id"},
	}
}

// Permission declares default permission behavior for this tool.
func (t JobDeleteTool) Permission() Permission {
	return AutoApprove
}

// Execute deletes one scheduled job.
func (t JobDeleteTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	if t.Service == nil {
		return nil, errors.New("job service is required")
	}
	id, err := stringArg(args, "id")
	if err != nil {
		return nil, err
	}
	if err := t.Service.Delete(ctx, id); err != nil {
		return nil, err
	}
	return &ToolResult{Output: "deleted"}, nil
}

// JobRunTool executes a scheduled job immediately by ID.
type JobRunTool struct {
	Service *scheduler.Service
}

// Name returns the tool name.
func (t JobRunTool) Name() string {
	return "job_run"
}

// Description returns the tool description for the model.
func (t JobRunTool) Description() string {
	return "Run a scheduled job immediately by ID"
}

// Schema returns the JSON schema for job_run args.
func (t JobRunTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id": map[string]any{
				"type":        "string",
				"description": "Scheduled job ID",
			},
		},
		"required": []string{"id"},
	}
}

// Permission declares default permission behavior for this tool.
func (t JobRunTool) Permission() Permission {
	return AutoApprove
}

// Execute runs one scheduled job now and returns its output.
func (t JobRunTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	if t.Service == nil {
		return nil, errors.New("scheduler service is required")
	}
	id, err := stringArg(args, "id")
	if err != nil {
		return nil, err
	}
	output, err := t.Service.RunNow(ctx, id)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(output) == "" {
		return &ToolResult{Output: "ok"}, nil
	}
	return TruncateOutput(output)
}

func objectArg(args map[string]any, key string) (map[string]any, error) {
	v, ok := args[key]
	if !ok {
		return nil, fmt.Errorf("missing required argument %s", key)
	}
	obj, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("argument %s must be an object", key)
	}
	out := make(map[string]any, len(obj))
	for k, value := range obj {
		out[k] = value
	}
	return out, nil
}

func validateJobAction(action scheduler.Action) error {
	switch action {
	case scheduler.ActionSendMessage, scheduler.ActionRunCommand, scheduler.ActionHTTPRequest:
		return nil
	default:
		return fmt.Errorf("unsupported job action %s", action)
	}
}
