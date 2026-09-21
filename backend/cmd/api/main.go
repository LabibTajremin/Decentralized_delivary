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

	cartapp "github.com/rootlogic-lab/delivery/backend/internal/modules/cart/application"
	cartpg "github.com/rootlogic-lab/delivery/backend/internal/modules/cart/infrastructure/persistence/postgres"
	carthttp "github.com/rootlogic-lab/delivery/backend/internal/modules/cart/transport/http"
	catapp "github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/application"
	catpg "github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/infrastructure/persistence/postgres"
	cathttp "github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/transport/http"
	cfgapp "github.com/rootlogic-lab/delivery/backend/internal/modules/config/application"
	cfgpg "github.com/rootlogic-lab/delivery/backend/internal/modules/config/infrastructure/persistence/postgres"
	cfghttp "github.com/rootlogic-lab/delivery/backend/internal/modules/config/transport/http"
	discoapp "github.com/rootlogic-lab/delivery/backend/internal/modules/discovery/application"
	discohttp "github.com/rootlogic-lab/delivery/backend/internal/modules/discovery/transport/http"
	dispatchapp "github.com/rootlogic-lab/delivery/backend/internal/modules/dispatch/application"
	dispatchpg "github.com/rootlogic-lab/delivery/backend/internal/modules/dispatch/infrastructure/persistence/postgres"
	dispatchhttp "github.com/rootlogic-lab/delivery/backend/internal/modules/dispatch/transport/http"
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
	merchantapp "github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/application"
	merchantpg "github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/infrastructure/persistence/postgres"
	merchanthttp "github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/transport/http"
	notificationapp "github.com/rootlogic-lab/delivery/backend/internal/modules/notification/application"
	notificationpg "github.com/rootlogic-lab/delivery/backend/internal/modules/notification/infrastructure/persistence/postgres"
	notificationpush "github.com/rootlogic-lab/delivery/backend/internal/modules/notification/infrastructure/push/log"
	notificationsms "github.com/rootlogic-lab/delivery/backend/internal/modules/notification/infrastructure/sms/log"
	notificationhttp "github.com/rootlogic-lab/delivery/backend/internal/modules/notification/transport/http"
	orderapp "github.com/rootlogic-lab/delivery/backend/internal/modules/order/application"
	ordercodes "github.com/rootlogic-lab/delivery/backend/internal/modules/order/infrastructure/codes"
	orderpg "github.com/rootlogic-lab/delivery/backend/internal/modules/order/infrastructure/persistence/postgres"
	orderhttp "github.com/rootlogic-lab/delivery/backend/internal/modules/order/transport/http"
	paymentapp "github.com/rootlogic-lab/delivery/backend/internal/modules/payment/application"
	paymentorderx "github.com/rootlogic-lab/delivery/backend/internal/modules/payment/external/order"
	paymentmanual "github.com/rootlogic-lab/delivery/backend/internal/modules/payment/infrastructure/gateway/manual"
	paymentpg "github.com/rootlogic-lab/delivery/backend/internal/modules/payment/infrastructure/persistence/postgres"
	paymenthttp "github.com/rootlogic-lab/delivery/backend/internal/modules/payment/transport/http"
	pricingapp "github.com/rootlogic-lab/delivery/backend/internal/modules/pricing/application"
	reviewapp "github.com/rootlogic-lab/delivery/backend/internal/modules/review/application"
	reviewpg "github.com/rootlogic-lab/delivery/backend/internal/modules/review/infrastructure/persistence/postgres"
	reviewhttp "github.com/rootlogic-lab/delivery/backend/internal/modules/review/transport/http"
	trackingapp "github.com/rootlogic-lab/delivery/backend/internal/modules/tracking/application"
	trackinghttp "github.com/rootlogic-lab/delivery/backend/internal/modules/tracking/transport/http"
	userapp "github.com/rootlogic-lab/delivery/backend/internal/modules/user/application"
	userpg "github.com/rootlogic-lab/delivery/backend/internal/modules/user/infrastructure/persistence/postgres"
	userhttp "github.com/rootlogic-lab/delivery/backend/internal/modules/user/transport/http"
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
		geoapp.NewPlaceMerchantUseCase(geoRepo),
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
	// identityService exposes IdentityContract to other modules — until P14,
	// nothing needed to reach identity this way; notification's SMS fallback
	// (PhoneFor) is the first real caller.
	identityService := identityapp.NewService(signer, sessionStore, directory)
	cfgResolve := cfgapp.NewResolveUseCase(cfgRepo)
	configService := cfgapp.NewService(cfgResolve)

	identityhttp.NewHandler(
		identityapp.NewRequestOTPUseCase(otpStore, limiter, smsSender, configService, systemClock, nil, logger),
		identityapp.NewVerifyOTPUseCase(otpStore, sessionStore, signer, directory, limiter, configService, systemClock, ids, nil, logger),
		identityapp.NewRefreshSessionUseCase(sessionStore, signer, limiter, configService, systemClock, ids, nil, logger),
		identityapp.NewLogoutUseCase(sessionStore, logger),
		identityapp.NewListSessionsUseCase(sessionStore),
		authenticator,
	).Register(mux)

	// Config (P03/P15), behind the admin role. Mounted after identity because
	// it needs the authenticator; the guard is passed in so config never
	// depends on identity's transport package.
	cfgSet := cfgapp.NewSetOverrideUseCase(cfgRepo, systemClock, ids)
	cfghttp.NewHandler(
		cfgResolve,
		cfgSet,
		cfgapp.NewClearOverrideUseCase(cfgRepo, systemClock, ids),
		cfgapp.NewListChangesUseCase(cfgRepo),
		cfgapp.NewAutoTuneUseCase(cfgResolve, cfgSet),
		cfghttp.Guard(authenticator.Require(identitydomain.RoleAdmin)),
		func(r *http.Request) (string, bool) {
			principal, ok := identityhttp.PrincipalFrom(r.Context())
			return principal.UserID, ok
		},
	).Register(mux)

	// User profile and addresses (P05). Every route acts on the caller's own
	// data, so the user id comes from the verified token rather than the
	// request.
	userRepo := userpg.NewFromPool(pool)
	userProfiles := userapp.NewProfileUseCase(userRepo)
	userAddresses := userapp.NewAddressUseCase(userRepo, geoService, ids)
	userService := userapp.NewService(userProfiles, userAddresses)
	userhttp.NewHandler(
		userProfiles,
		userAddresses,
		userhttp.Guard(authenticator.Authenticated()),
		func(r *http.Request) (string, bool) {
			principal, ok := identityhttp.PrincipalFrom(r.Context())
			return principal.UserID, ok
		},
	).Register(mux)

	// Merchant (P06). Registration is open to any signed-in account — D1 says a
	// shop may register from anywhere in Bangladesh, and the merchant role is
	// something an approved owner gets, not a prerequisite for applying. The
	// approval queue sits behind the admin role.
	merchantRepo := merchantpg.NewFromPool(pool)
	merchantService := merchantapp.NewService(merchantRepo, systemClock)
	merchanthttp.NewHandler(
		merchantapp.NewRegistrationUseCase(merchantRepo, geoService, systemClock, ids),
		merchantapp.NewOperationsUseCase(merchantRepo, systemClock),
		merchantapp.NewModerationUseCase(merchantRepo, geoService, systemClock, ids),
		systemClock,
		merchanthttp.Guard(authenticator.Authenticated()),
		merchanthttp.Guard(authenticator.Require(identitydomain.RoleAdmin)),
		func(r *http.Request) (string, bool) {
			principal, ok := identityhttp.PrincipalFrom(r.Context())
			return principal.UserID, ok
		},
	).Register(mux)

	// Catalogue (P07). The owner routes sit behind authentication and check
	// ownership against the merchant record; the customer's menu read is
	// public, because browsing is what the app does before anyone signs in.
	catRepo := catpg.NewFromPool(pool)
	catalogueService := catapp.NewService(catRepo, systemClock)
	cathttp.NewHandler(
		catapp.NewCategoryUseCase(catRepo, catRepo, merchantService, ids),
		catapp.NewItemUseCase(catRepo, catRepo, merchantService, systemClock, ids),
		catapp.NewOptionUseCase(catRepo, merchantService, ids),
		catapp.NewComboUseCase(catRepo, catRepo, merchantService, ids),
		catalogueService,
		cathttp.Guard(authenticator.Authenticated()),
		func(r *http.Request) (string, bool) {
			principal, ok := identityhttp.PrincipalFrom(r.Context())
			return principal.UserID, ok
		},
	).Register(mux)

	// Pricing (P10). ALG-05 lives here and nowhere else: discovery quotes a fee
	// on every shop card and the cart quotes one at checkout, and both go
	// through this service so the two numbers can never disagree.
	pricingService := pricingapp.NewService(configService)

	// Discovery (P08). D1's local visibility, D2's expansion and D3's ceiling.
	// It owns no storage: geo says where shops are, merchant says which may be
	// seen, and config says how far the customer can look from here. The fee
	discoveryReach := discoapp.NewReachUseCase(geoService, merchantService, configService)
	discoveryService := discoapp.NewService(discoveryReach)
	discohttp.NewHandler(
		discoapp.NewSearchUseCase(geoService, merchantService, configService, pricingService),
		discoveryReach,
	).Register(mux)

	// Cart (P09). One cart per customer, from one shop, held against one
	// delivery address. Every read revalidates against the shop as it stands —
	// the menu has been edited since, and the address may have moved into
	// another division, which D3 makes permanent.
	cartRepo := cartpg.NewFromPool(pool)
	cartUseCase := cartapp.NewCartUseCase(
		cartRepo, catalogueService, merchantService, discoveryService, pricingService, systemClock, ids,
	)
	cartService := cartapp.NewService(cartUseCase)
	carthttp.NewHandler(
		cartUseCase,
		carthttp.Guard(authenticator.Authenticated()),
		func(r *http.Request) (string, bool) {
			principal, ok := identityhttp.PrincipalFrom(r.Context())
			return principal.UserID, ok
		},
	).Register(mux)

	// Order (P11). The lifecycle lives in one transition table, and every
	// party — customer, shop, rider, admin — reaches it through the same use
	// case, so "who may do this" cannot drift between four endpoints. Prices
	// are re-quoted on the way in rather than taken from the cart: a price the
	// customer was shown is not a price the system promised.
	// Dispatch (P12). Built before order because order hands it a job the
	// moment a shop marks food ready; dispatch in turn moves the order through
	// OrderContract.Advance, which is the way in the order module left for it.
	// The two are wired as a cycle of *services*, not of packages: each reaches
	// the other only through its external/ seam (2.5).
	dispatchRepo := dispatchpg.NewFromPool(pool)
	orderRepo := orderpg.NewFromPool(pool)
	orderTransitions := orderapp.NewTransitionUseCase(
		orderRepo, merchantService, configService, nil, systemClock, ids,
	)
	orderService := orderapp.NewService(orderRepo, orderTransitions)

	dispatchOffers := dispatchapp.NewOfferUseCase(dispatchRepo, configService, systemClock, ids)
	dispatchService := dispatchapp.NewService(dispatchRepo, dispatchOffers)
	// The order module's half of the pair, filled in once dispatch exists.
	orderTransitions.UseDispatch(dispatchService)

	// Payment (P13). The isolated module (2.6): it talks to order and dispatch
	// only through their own contracts, mirrored into payment's own
	// primitives at the seam, and exposes only PaymentContract to everyone
	// else. The manual gateway is a stand-in until a real provider is chosen —
	// it refuses to be constructed in production, the same guarantee
	// identity's log SMS sender makes.
	paymentRepo := paymentpg.NewFromPool(pool)
	manualGateway, err := paymentmanual.New(cfg.PaymentWebhookSecret, cfg.IsProduction())
	if err != nil {
		return nil, err
	}
	paymentCollections := paymentapp.NewCollectionUseCase(
		paymentRepo, dispatchService, systemClock, ids,
	)
	paymentRefunds := paymentapp.NewRefundUseCase(paymentRepo, manualGateway, systemClock)
	paymentService := paymentapp.NewService(paymentRepo, paymentCollections, paymentRefunds)
	// The order module's half of the payment pair, filled in once payment
	// exists — the same service-level cycle as order and dispatch above.
	orderTransitions.UsePayment(paymentService)

	// Notification (P14). Push first, falling back to SMS — both log-only
	// stand-ins until a real provider is chosen, both refusing to be
	// constructed in production for the same reason identity's log SMS
	// sender does: a message written to the log is readable by anyone with
	// log access. identityService supplies the one lookup a fallback needs
	// (PhoneFor) directly, with no adapter, the same as dispatchService and
	// orderService are handed to other modules' seams below.
	notificationRepo := notificationpg.NewFromPool(pool)
	notificationPush, err := notificationpush.New(logger, cfg.IsProduction())
	if err != nil {
		return nil, err
	}
	notificationSMS, err := notificationsms.New(logger, cfg.IsProduction())
	if err != nil {
		return nil, err
	}
	notificationNotify := notificationapp.NewNotifyUseCase(
		notificationRepo, notificationPush, notificationSMS, identityService, systemClock, ids,
	)
	notificationService := notificationapp.NewService(notificationNotify)
	// Not a service-level cycle — notification never calls back into
	// order — but wired the same way as dispatch and payment for one
	// consistent pattern.
	orderTransitions.UseNotification(notificationService)

	dispatchhttp.NewHandler(
		dispatchapp.NewPartnerUseCase(
			dispatchRepo, orderService, geoService, configService, systemClock, ids,
		),
		dispatchOffers,
		dispatchhttp.Guard(authenticator.Authenticated()),
		dispatchhttp.Guard(authenticator.Require(identitydomain.RoleAdmin)),
		func(r *http.Request) (string, bool) {
			principal, ok := identityhttp.PrincipalFrom(r.Context())
			return principal.UserID, ok
		},
	).Register(mux)
	orderhttp.NewHandler(
		orderapp.NewPlaceUseCase(
			orderRepo, ordercodes.New(), cartService, pricingService,
			merchantService, userService, discoveryService, configService,
			systemClock, ids,
		),
		orderTransitions,
		orderapp.NewReadUseCase(orderRepo, configService, systemClock),
		orderhttp.Guard(authenticator.Authenticated()),
		orderhttp.Guard(authenticator.Require(identitydomain.RoleAdmin)),
		func(r *http.Request) (string, bool) {
			principal, ok := identityhttp.PrincipalFrom(r.Context())
			return principal.UserID, ok
		},
		func(r *http.Request, userID string) ([]string, error) {
			return merchantService.OwnedBy(r.Context(), userID)
		},
	).Register(mux)

	orderOfPayment := paymentorderx.New(orderService)
	paymenthttp.NewHandler(
		paymentapp.NewCheckoutUseCase(paymentRepo, manualGateway, orderOfPayment, systemClock, ids),
		paymentapp.NewWebhookUseCase(paymentRepo, manualGateway, orderOfPayment, systemClock),
		paymentCollections,
		paymentRefunds,
		paymentapp.NewReadUseCase(paymentRepo),
		paymenthttp.Guard(authenticator.Authenticated()),
		paymenthttp.Guard(authenticator.Require(identitydomain.RoleAdmin)),
		func(r *http.Request) (string, bool) {
			principal, ok := identityhttp.PrincipalFrom(r.Context())
			return principal.UserID, ok
		},
		!cfg.IsProduction(),
	).Register(mux)

	notificationhttp.NewHandler(
		notificationapp.NewRegisterDeviceUseCase(notificationRepo),
		notificationapp.NewReadUseCase(notificationRepo),
		notificationhttp.Guard(authenticator.Authenticated()),
		func(r *http.Request) (string, bool) {
			principal, ok := identityhttp.PrincipalFrom(r.Context())
			return principal.UserID, ok
		},
	).Register(mux)

	// Tracking (P14). Owns no storage of its own: order already has the
	// status and the history, dispatch already has where the rider is, and
	// orderService/dispatchService are handed to its external/ seams
	// directly — both already implement methods of the exact shape tracking
	// needs, with no adapter.
	trackinghttp.NewHandler(
		trackingapp.NewSnapshotUseCase(orderService, dispatchService),
		cfg.TrackingStreamInterval,
		trackinghttp.Guard(authenticator.Authenticated()),
		func(r *http.Request) (string, bool) {
			principal, ok := identityhttp.PrincipalFrom(r.Context())
			return principal.UserID, ok
		},
	).Register(mux)

	// Review & support (P16). Owns no storage but its own two small tables:
	// eligibility for a review, and who a ticket's refund goes to, both come
	// from OrderContract and PaymentContract — orderService and
	// paymentService are handed to its external/ seams directly, the same
	// zero-adapter pattern every phase since P13 has used where the target's
	// own method already matches the shape a consumer needs.
	reviewRepo := reviewpg.NewFromPool(pool)
	reviewRatings := reviewapp.NewRatingsUseCase(reviewRepo)
	reviewhttp.NewHandler(
		reviewapp.NewSubmitReviewUseCase(reviewRepo, orderService, systemClock, ids),
		reviewRatings,
		reviewapp.NewListReviewsUseCase(reviewRepo),
		reviewapp.NewRaiseTicketUseCase(reviewRepo, orderService, systemClock, ids),
		reviewapp.NewResolveTicketUseCase(reviewRepo, paymentService, systemClock),
		reviewapp.NewMyTicketsUseCase(reviewRepo),
		reviewapp.NewOpenTicketsUseCase(reviewRepo),
		reviewhttp.Guard(authenticator.Authenticated()),
		reviewhttp.Guard(authenticator.Require(identitydomain.RoleAdmin)),
		func(r *http.Request) (string, bool) {
			principal, ok := identityhttp.PrincipalFrom(r.Context())
			return principal.UserID, ok
		},
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
