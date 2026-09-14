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
	goredis "github.com/redis/go-redis/v9"

	cfgapp "github.com/rootlogic-lab/delivery/backend/internal/modules/config/application"
	cfgpg "github.com/rootlogic-lab/delivery/backend/internal/modules/config/infrastructure/persistence/postgres"
	cfghttp "github.com/rootlogic-lab/delivery/backend/internal/modules/config/transport/http"
	geoapp "github.com/rootlogic-lab/delivery/backend/internal/modules/geo/application"
	geopg "github.com/rootlogic-lab/delivery/backend/internal/modules/geo/infrastructure/persistence/postgres"
	geohttp "github.com/rootlogic-lab/delivery/backend/internal/modules/geo/transport/http"
	identityapp "github.com/rootlogic-lab/delivery/backend/internal/modules/identity/application"
	identitydomain "github.com/rootlogic-lab/delivery/backend/internal/modules/identity/domain"
	identitypg "github.com/rootlogic-lab/delivery/backend/internal/modules/identity/infrastructure/persistence/postgres"
	identityredis "github.com/rootlogic-lab/delivery/backend/internal/modules/identity/infrastructure/redis"
	identitysms "github.com/rootlogic-lab/delivery/backend/internal/modules/identity/infrastructure/sms"
	identitytoken "github.com/rootlogic-lab/delivery/backend/internal/modules/identity/infrastructure/token"
	identityhttp "github.com/rootlogic-lab/delivery/backend/internal/modules/identity/transport/http"
	"github.com/rootlogic-lab/delivery/backend/internal/platform/assets"
	"github.com/rootlogic-lab/delivery/backend/internal/platform/httpx"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/clock"
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

	redisClient := goredis.NewClient(redisOptions(cfg.RedisURL, logger))
	defer func() { _ = redisClient.Close() }()

	handler, err := buildRouter(cfg, logger, pool, redisClient)
	if err != nil {
		return err
	}

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

// redisOptions parses the Redis URL, falling back to a local default so a
// malformed URL is a loud log line rather than a nil client that panics on the
// first request.
func redisOptions(url string, logger *slog.Logger) *goredis.Options {
	opts, err := goredis.ParseURL(url)
	if err != nil {
		logger.Error("REDIS_URL is not a valid redis:// or rediss:// URL", "error", err)
		return &goredis.Options{Addr: "127.0.0.1:6379"}
	}
	return opts
}

// buildRouter mounts every module's transport behind the shared middleware.
func buildRouter(cfg config.Config, logger *slog.Logger, pool *pgxpool.Pool, redisClient *goredis.Client) (http.Handler, error) {
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

	// Config (P03). The business rules an operator tunes at runtime, as
	// distinct from the deploy-time settings in config.Config above.
	ids := id.NewGen(nil, nil)
	systemClock := clock.System{}
	cfgRepo := cfgpg.NewFromPool(pool)

	// Identity (P04). Sessions and one-time codes live in Redis, where TTLs
	// expire them without a sweeper job; only the account record is in
	// Postgres (ADR 0005).
	signer, err := identitytoken.NewSigner([]byte(cfg.JWTSigningKey), cfg.JWTIssuer)
	if err != nil {
		return nil, err
	}
	smsSender, err := identitysms.NewLogSender(logger, cfg.IsProduction())
	if err != nil {
		// A production deployment with no SMS gateway configured must not
		// start: it would accept sign-ins and write every code to the log.
		return nil, err
	}
	authenticator := identityhttp.NewAuthenticator(signer)
	otpStore := identityredis.NewOTPStore(redisClient)
	sessionStore := identityredis.NewSessionStore(redisClient)
	limiter := identityredis.NewRateLimiter(redisClient)
	directory := identitypg.NewFromPool(pool, ids)
	configService := cfgapp.NewService(cfgapp.NewResolveUseCase(cfgRepo))

	identityhttp.NewHandler(
		identityapp.NewRequestOTPUseCase(otpStore, limiter, smsSender, configService, systemClock, nil, logger),
		identityapp.NewVerifyOTPUseCase(otpStore, sessionStore, signer, directory, limiter, configService, systemClock, ids, nil, logger),
		identityapp.NewRefreshSessionUseCase(sessionStore, signer, limiter, configService, systemClock, ids, nil, logger),
		identityapp.NewLogoutUseCase(sessionStore, logger),
		identityapp.NewListSessionsUseCase(sessionStore),
		authenticator,
	).Register(mux)

	// Config (P03), behind the admin role. Mounted after identity because it
	// needs the authenticator; the guard is passed in so config never depends
	// on identity's transport package.
	cfghttp.NewHandler(
		cfgapp.NewResolveUseCase(cfgRepo),
		cfgapp.NewSetOverrideUseCase(cfgRepo, systemClock, ids),
		cfgapp.NewClearOverrideUseCase(cfgRepo, systemClock, ids),
		cfghttp.Guard(authenticator.Require(identitydomain.RoleAdmin)),
	).Register(mux)

	// Demo imagery is served only outside production, so generated placeholder
	// logos can never appear beside real merchants.
	if demo, demoErr := assets.Handler(cfg.IsProduction()); demoErr == nil {
		mux.Handle(assets.Prefix, demo)
		logger.Info("serving demo assets", "prefix", assets.Prefix)
	}

	return httpx.Chain(mux,
		httpx.RequestID(ids),
		httpx.Recover(logger),
		httpx.Logging(logger),
		httpx.SecurityHeaders(),
		httpx.CORS(cfg.CORSAllowedOrigins),
	), nil
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
