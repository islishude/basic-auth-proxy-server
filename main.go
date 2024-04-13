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
				r.Host = remote.Host
				p.ServeHTTP(w, r)
			} else {
				w.Header().Set("WWW-Authenticate", "Basic")
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			}
		}
	}

	http.HandleFunc("/-/ready", func(w http.ResponseWriter, r *http.Request) {
		readiness := remote.JoinPath(config.Backend.Readiness)
		resp, err := http.Get(readiness.String())
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
