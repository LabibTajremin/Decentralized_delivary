// Package main wires the delivery API together and starts the HTTP server.
//
// Wiring only: this file constructs concrete implementations and hands them to
// the application layer. It contains no business rules, which is why it is the
// single permitted coverage exclusion for the backend module (see
// backend/tests/coverage-exclusions.txt). Its behaviour is covered by the E2E
// suite in backend/tests/e2e.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/platform/assets"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/config"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load(nil)
	if err != nil {
		logger.Error("configuration", "error", err)
		os.Exit(1)
	}
	addr := cfg.APIAddr

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"status":"ok"}`)
	})

	// Demo imagery is served only outside production, so generated placeholder
	// logos can never appear beside real merchants.
	if demo, demoErr := assets.Handler(cfg.IsProduction()); demoErr == nil {
		mux.Handle(assets.Prefix, demo)
		logger.Info("serving demo assets", "prefix", assets.Prefix)
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("api listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownGap)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
}
