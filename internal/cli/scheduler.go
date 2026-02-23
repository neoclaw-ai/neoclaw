package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/machinae/betterclaw/internal/approval"
	"github.com/machinae/betterclaw/internal/config"
	"github.com/machinae/betterclaw/internal/sandbox"
	"github.com/machinae/betterclaw/internal/scheduler"
	"github.com/machinae/betterclaw/internal/tools"
)

func newSchedulerService(cfg *config.Config, channelWriters map[string]io.Writer) (*scheduler.Service, error) {
	runner, err := newSchedulerRunner(cfg, channelWriters)
	if err != nil {
		return nil, err
	}
	return scheduler.NewService(cfg.JobsPath(), runner), nil
}

func newSchedulerRunner(cfg *config.Config, channelWriters map[string]io.Writer) (*scheduler.Runner, error) {
	proxyAddress := ""
	if cfg.Security.Mode != config.SecurityModeDanger {
		domainProxy, err := sandbox.StartDomainProxy(approval.Checker{
			AllowedDomainsPath: cfg.AllowedDomainsPath(),
		})
		if err != nil {
			return nil, fmt.Errorf("start scheduler domain proxy: %w", err)
		}
		proxyAddress = domainProxy.Addr()
	}

	httpClient := &http.Client{
		Transport: approval.RoundTripper{
			Checker: approval.Checker{
				AllowedDomainsPath: cfg.AllowedDomainsPath(),
			},
		},
	}

	runTool := tools.RunCommandTool{
		WorkspaceDir: cfg.WorkspaceDir(),
		Timeout:      cfg.Security.CommandTimeout,
		SecurityMode: cfg.Security.Mode,
		ProxyAddress: proxyAddress,
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
	}, channelWriters), nil
}
