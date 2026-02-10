package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	var confPath string
	flag.StringVar(&confPath, "config", "config.yaml", "config path")
	flag.Parse()

	slog.Info("parsing config", "path", confPath)
	config, err := ParseConfig(confPath)
	if err != nil {
		slog.Error("failed to parse config", "path", confPath, "err", err)
		os.Exit(1)
	}

	remote, err := url.Parse(config.Backend.Target)
	if err != nil {
		slog.Error("failed to parse remote server", "backend", config.Backend, "err", err)
		os.Exit(1)
	}

	mux := newServeMux(config, remote)

	slog.Info("proxying", "server", remote, "port", config.Port)
	server := http.Server{Addr: fmt.Sprintf(":%d", config.Port), Handler: mux}

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
