package httpx

import (
	"context"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/id"
)

// Middleware wraps a handler.
type Middleware func(http.Handler) http.Handler

// Chain applies middleware so the first listed is the outermost.
//
// Order is written the way it executes, because a stack whose order reads
// backwards is a stack people get wrong: recovery must wrap logging so a panic
// is still logged, and request IDs must be set before either.
func Chain(h http.Handler, middlewares ...Middleware) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}

type ctxKey int

const requestIDKey ctxKey = iota

// RequestIDHeader carries the correlation id in and out.
const RequestIDHeader = "X-Request-Id"

// RequestIDFrom returns the request id, or "" outside a request.
func RequestIDFrom(ctx context.Context) string {
	v, _ := ctx.Value(requestIDKey).(string)
	return v
}

// WithRequestID puts a request id on the context. Exported for tests and for
// background work that continues after a request.
func WithRequestID(ctx context.Context, rid string) context.Context {
	return context.WithValue(ctx, requestIDKey, rid)
}

// RequestID attaches a correlation id to every request and echoes it back.
//
// An inbound id is accepted only if it is entirely well-formed. It ends up in
// log lines, so a caller who can put newlines in it can forge log entries and
// hide their own activity — and a value that is partly hostile is not worth
// salvaging, so anything unexpected is discarded and a fresh id generated
// instead. That costs a gateway its trace correlation only when the gateway is
// sending something it should not.
//
// The generator is injected so a test gets deterministic ids without the
// production path losing its entropy source.
func RequestID(gen id.Generator) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rid := r.Header.Get(RequestIDHeader)
			if !validRequestID(rid) {
				rid = gen.New("req")
			}
			w.Header().Set(RequestIDHeader, rid)
			next.ServeHTTP(w, r.WithContext(WithRequestID(r.Context(), rid)))
		})
	}
}

const maxRequestIDLen = 64

// validRequestID reports whether an inbound id is safe to log verbatim.
func validRequestID(raw string) bool {
	if raw == "" || len(raw) > maxRequestIDLen {
		return false
	}
	for _, r := range raw {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}

// statusRecorder captures the status and size for the access log.
type statusRecorder struct {
	http.ResponseWriter
	status  int
	written int64
}

func (s *statusRecorder) WriteHeader(status int) {
	if s.status == 0 {
		s.status = status
		s.ResponseWriter.WriteHeader(status)
	}
}

// Flush forwards to the wrapped writer when it can — embedding
// http.ResponseWriter as an interface field promotes only that interface's own
// methods, not Flush, so without this a streaming response (tracking's SSE
// endpoint, P14) would sit fully buffered behind Logging until the handler
// returned, defeating the entire point of a stream.
func (s *statusRecorder) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	n, err := s.ResponseWriter.Write(b)
	s.written += int64(n)
	return n, err
}

// Recover turns a panic into a 500 instead of a dropped connection.
//
// A panic in one handler must not take the process down: the other requests in
// flight have done nothing wrong. The stack reaches the log; the client gets
// the same opaque message as any other internal error.
func Recover(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Error("panic in handler",
						"error", rec,
						"method", r.Method,
						"path", r.URL.Path,
						"request_id", RequestIDFrom(r.Context()),
					)
					WriteError(w, errs.New(errs.KindInternal, "internal_error", internalMessage))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// Logging writes one structured line per request.
//
// Client errors log at info and server errors at error, so an alert on error
// lines fires for our bugs and not for someone sending a bad postcode.
func Logging(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w}
			next.ServeHTTP(rec, r)
			if rec.status == 0 {
				rec.status = http.StatusOK
			}

			level := slog.LevelInfo
			if rec.status >= http.StatusInternalServerError {
				level = slog.LevelError
			}
			logger.Log(r.Context(), level, "http request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.status,
				"bytes", rec.written,
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", RequestIDFrom(r.Context()),
			)
		})
	}
}

// CORS allows exactly the configured origins.
//
// There is no wildcard and no reflect-any-origin path. The apps are native and
// need none of this; only the admin and merchant web panels do, and those have
// known hostnames. Reflecting an arbitrary Origin would let any website make
// credentialed calls on a signed-in admin's behalf.
func CORS(allowedOrigins []string) Middleware {
	allowed := slices.Clone(allowedOrigins)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && slices.Contains(allowed, origin) {
				h := w.Header()
				h.Set("Access-Control-Allow-Origin", origin)
				h.Set("Access-Control-Allow-Credentials", "true")
				h.Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
				h.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, "+RequestIDHeader)
				h.Set("Access-Control-Max-Age", "600")
				// Responses differ by Origin, so a shared cache must not serve
				// one origin's response to another.
				h.Add("Vary", "Origin")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// SecurityHeaders sets the headers that cost nothing and close real holes.
func SecurityHeaders() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			h.Set("Referrer-Policy", "no-referrer")
			next.ServeHTTP(w, r)
		})
	}
}
