package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	var (
		ConfPath string
	)
	flag.StringVar(&ConfPath, "config", "config.yaml", "config path")
	flag.Parse()

	slog.Info("parse config", "path", ConfPath)
	config, err := ParseConfig(ConfPath)
	if err != nil {
		slog.Error("failed to parse config", "path", ConfPath, "err", err)
		os.Exit(1)
	}

	remote, err := url.Parse(config.Backend.Target)
	if err != nil {
		slog.Error("failed to remote server", "backend", config.Backend, "err", err)
		os.Exit(1)
	}

	handler := func(p *httputil.ReverseProxy) func(http.ResponseWriter, *http.Request) {
		return func(w http.ResponseWriter, r *http.Request) {
			user, pass, ok := r.BasicAuth()
			if ok && VerifyUserPass(config.Users, user, pass) {
				slog.Info("New request", "user", user, "host", r.Host, "path", r.URL.Path)
				r.Host = remote.Host
				p.ServeHTTP(w, r)
			} else {
				w.Header().Set("WWW-Authenticate", "Basic")
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			}
		}
	}

	http.HandleFunc("GET /-/", func(w http.ResponseWriter, r *http.Request) {
		var target string
		switch r.URL.Path {
		case "/-/ready":
			if config.Backend.Readiness == "" {
				return
			}
			target = remote.JoinPath(config.Backend.Readiness).String()
		case "/-/healthy":
			if config.Backend.Liveness == "" {
				return
			}
			target = remote.JoinPath(config.Backend.Liveness).String()
		default:
			return
		}

		timeout := 3 * time.Second
		if t := config.Backend.HealthCheckTimeoutInSecond; t != 0 {
			timeout = time.Duration(t) * time.Second
		}

		newctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		req, err := http.NewRequestWithContext(newctx, http.MethodGet, target, nil)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
			return
		}

		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
	})

	http.HandleFunc("/", handler(httputil.NewSingleHostReverseProxy(remote)))

	slog.Info("proxying", "server", remote, "port", config.Port)
	server := http.Server{Addr: fmt.Sprintf(":%d", config.Port), Handler: http.DefaultServeMux}

	basectx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	go func() {
		defer cancel()
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("ListenAndServe", "err", err)
		}
	}()

	<-basectx.Done()
	slog.Info("stopping")
	_ = server.Shutdown(context.Background())
}
