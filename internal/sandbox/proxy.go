package sandbox

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/elazarl/goproxy"
	"github.com/machinae/betterclaw/internal/approval"
)

// DomainProxy enforces outbound domain policy for subprocess HTTP(S) traffic.
type DomainProxy struct {
	server *http.Server
	addr   string
}

// Addr returns the proxy listen address as an HTTP URL.
func (p *DomainProxy) Addr() string {
	if p == nil {
		return ""
	}
	return p.addr
}

// Close stops the proxy server.
func (p *DomainProxy) Close() error {
	if p == nil || p.server == nil {
		return nil
	}
	return p.server.Close()
}

// StartDomainProxy starts a local HTTP proxy that applies the domain approval checker.
func StartDomainProxy(checker approval.Checker) (*DomainProxy, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen domain proxy: %w", err)
	}

	proxy := goproxy.NewProxyHttpServer()
	proxy.Verbose = false
	proxy.OnRequest().HandleConnectFunc(func(host string, _ *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
		if err := checker.Allow(context.Background(), host); err != nil {
			return goproxy.RejectConnect, host
		}
		return goproxy.OkConnect, host
	})
	proxy.OnRequest().DoFunc(func(req *http.Request, _ *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		if req == nil || req.URL == nil {
			return req, nil
		}
		if err := checker.Allow(context.Background(), req.URL.Host); err != nil {
			return req, goproxy.NewResponse(req, goproxy.ContentTypeText, http.StatusForbidden, err.Error())
		}
		return req, nil
	})

	server := &http.Server{Handler: proxy}
	go func() {
		_ = server.Serve(ln)
	}()

	return &DomainProxy{
		server: server,
		addr:   "http://" + ln.Addr().String(),
	}, nil
}
