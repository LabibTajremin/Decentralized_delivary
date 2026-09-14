// Package main wires the delivery API together and starts the HTTP server.
//
// Wiring only: this file constructs concrete implementations and hands them to
// the application layer. It contains no business rules, which is why it is a
// permitted coverage exclusion (see backend/tests/coverage-exclusions.txt). Its
// behaviour is covered by the E2E suite in backend/tests/e2e.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	geoapp "github.com/rootlogic-lab/delivery/backend/internal/modules/geo/application"
	geopg "github.com/rootlogic-lab/delivery/backend/internal/modules/geo/infrastructure/persistence/postgres"
	geohttp "github.com/rootlogic-lab/delivery/backend/internal/modules/geo/transport/http"
	"github.com/rootlogic-lab/delivery/backend/internal/platform/assets"
	"github.com/rootlogic-lab/delivery/backend/internal/platform/httpx"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/config"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/id"
)

func main() {
	if err := run(); err != nil {
		slog.Default().Error("startup failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(nil)
	if err != nil {
		return err
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel(cfg.LogLevel),
	}))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// The pool is created eagerly but not dialled: pgxpool connects lazily, so
	// a database that is briefly unavailable at boot does not stop the process
	// from starting and answering its liveness probe.
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	handler := buildRouter(cfg, logger, pool)

	srv := &http.Server{
		Addr:              cfg.APIAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("api listening", "addr", cfg.APIAddr, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownGap)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// buildRouter mounts every module's transport behind the shared middleware.
func buildRouter(cfg config.Config, logger *slog.Logger, pool *pgxpool.Pool) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Geo (P02).
	geoRepo := geopg.NewFromPool(pool)
	geoService := geoapp.NewService(
		geoapp.NewResolveAreaUseCase(geoRepo),
		geoapp.NewMerchantsWithinRadiusUseCase(geoRepo),
	)
	geohttp.NewHandler(geoService).Register(mux)

	// Demo imagery is served only outside production, so generated placeholder
	// logos can never appear beside real merchants.
	if demo, demoErr := assets.Handler(cfg.IsProduction()); demoErr == nil {
		mux.Handle(assets.Prefix, demo)
		logger.Info("serving demo assets", "prefix", assets.Prefix)
	}

	return httpx.Chain(mux,
		httpx.RequestID(id.NewGen(nil, nil)),
		httpx.Recover(logger),
		httpx.Logging(logger),
		httpx.SecurityHeaders(),
		httpx.CORS(cfg.CORSAllowedOrigins),
	)
}

func logLevel(name string) slog.Level {
	switch name {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
