package cli

import (
	"context"
	"io"
	"net/http"
	"path/filepath"

	"github.com/machinae/betterclaw/internal/approval"
	"github.com/machinae/betterclaw/internal/config"
	"github.com/machinae/betterclaw/internal/scheduler"
	"github.com/machinae/betterclaw/internal/store"
	"github.com/machinae/betterclaw/internal/tools"
)

func newSchedulerService(cfg *config.Config, channelWriters map[string]io.Writer) *scheduler.Service {
	return scheduler.NewService(filepath.Join(cfg.AgentDir(), store.JobsFilePath), newSchedulerRunner(cfg, channelWriters))
}

func newSchedulerRunner(cfg *config.Config, channelWriters map[string]io.Writer) *scheduler.Runner {
	httpClient := &http.Client{
		Transport: approval.RoundTripper{
			Checker: approval.Checker{
				AllowedDomainsPath: filepath.Join(cfg.DataDir, store.AllowedDomainsFilePath),
			},
		},
	}

	runTool := tools.RunCommandTool{
		WorkspaceDir:    cfg.WorkspaceDir(),
		AllowedBinsPath: filepath.Join(cfg.DataDir, store.AllowedBinsFilePath),
		Timeout:         cfg.Security.CommandTimeout,
	}
	httpTool := tools.HTTPRequestTool{Client: httpClient}

	return scheduler.NewRunner(scheduler.ActionRunners{
		SendMessage: func(ctx context.Context, writer io.Writer, args map[string]any) (string, error) {
			sendTool := tools.SendMessageTool{Writer: writer}
			res, err := sendTool.Execute(ctx, args)
			if err != nil {
				return "", err
			}
			if res == nil {
				return "", nil
			}
			return res.Output, nil
		},
		RunCommand: func(ctx context.Context, args map[string]any) (string, error) {
			res, err := runTool.Execute(ctx, args)
			if err != nil {
				return "", err
			}
			if res == nil {
				return "", nil
			}
			return res.Output, nil
		},
		HTTPRequest: func(ctx context.Context, args map[string]any) (string, error) {
			res, err := httpTool.Execute(ctx, args)
			if err != nil {
				return "", err
			}
			if res == nil {
				return "", nil
			}
			return res.Output, nil
		},
	}, channelWriters)
}
