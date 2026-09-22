// Package e2e exercises the API as a running process.
//
// cmd/api is the one package excluded from the coverage gate
// (backend/tests/coverage-exclusions.txt) on the grounds that it is wiring
// covered by E2E. This file is what makes that claim true: it builds the real
// binary, runs it, and drives it over HTTP.
package e2e

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// buildAPI compiles cmd/api into a temporary directory and returns its path.
func buildAPI(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "api")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/api")
	cmd.Dir = "../../"
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build cmd/api: %v\n%s", err, out)
	}
	return bin
}

// freePort asks the kernel for an unused TCP port.
func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	defer func() { _ = l.Close() }()
	_, port, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		t.Fatalf("split port: %v", err)
	}
	return port
}

// startAPI runs the binary and waits until it answers, returning the base URL
// and a stop function that asserts a graceful shutdown.
//
// The environment passed here is the deployment contract: whatever the binary
// needs to start is what an operator has to set. Nothing is defaulted for the
// test's convenience, so a variable that becomes required shows up as a failure
// here rather than as a crash on someone's first deploy.
func startAPI(t *testing.T, bin string, extraEnv ...string) (string, func()) {
	t.Helper()
	return startAPIWithOutput(t, bin, nil, extraEnv...)
}

// startAPIWithOutput is startAPI with the server's stdout copied somewhere the
// test can read.
//
// The auth tests use it to pick up the one-time code the development SMS sender
// logs — the same route a developer takes to sign in locally, rather than a
// back door that exists only for tests.
func startAPIWithOutput(t *testing.T, bin string, output io.Writer, extraEnv ...string) (string, func()) {
	t.Helper()
	port := freePort(t)
	cmd := exec.Command(bin)
	if output != nil {
		cmd.Stdout = output
		cmd.Stderr = output
	}
	cmd.Env = append(cmd.Environ(),
		"API_ADDR=127.0.0.1:"+port,
		// The process does not connect to either at startup; they are required
		// configuration, and a missing one must fail fast rather than at the
		// first request.
		"DATABASE_URL=postgres://delivery:delivery@127.0.0.1:5432/delivery?sslmode=disable",
		"REDIS_URL=redis://127.0.0.1:6379/0",
	)
	cmd.Env = append(cmd.Env, extraEnv...)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start api: %v", err)
	}

	base := "http://127.0.0.1:" + port
	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(20 * time.Second)
	for {
		resp, err := client.Get(base + "/healthz")
		if err == nil {
			_ = resp.Body.Close()
			break
		}
		if time.Now().After(deadline) {
			_ = cmd.Process.Kill()
			t.Fatalf("api did not become ready: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	return base, func() {
		if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
			t.Errorf("signal api: %v", err)
			return
		}
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		select {
		case err := <-done:
			var exitErr *exec.ExitError
			if err != nil && !errors.As(err, &exitErr) {
				t.Errorf("api exited badly: %v", err)
			}
			if exitErr != nil {
				t.Errorf("api did not shut down cleanly: %v", exitErr)
			}
		case <-time.After(15 * time.Second):
			_ = cmd.Process.Kill()
			t.Error("api did not shut down within 15s of SIGTERM")
		}
	}
}

// TestHealthEndpoint asserts the liveness probe answers as the OpenAPI
// contract in api/openapi.yaml describes it.
func TestHealthEndpoint(t *testing.T) {
	base, stop := startAPI(t, buildAPI(t))
	defer stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/healthz", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Status != "ok" {
		t.Errorf("status field = %q, want %q", body.Status, "ok")
	}
}

// TestDemoAssetsAreServedOutsideProduction proves the images that ship in the
// binary are actually reachable over HTTP, which is what makes a fresh clone
// demonstrable with no CDN and no network.
func TestDemoAssetsAreServedOutsideProduction(t *testing.T) {
	base, stop := startAPI(t, buildAPI(t))
	defer stop()

	resp, err := http.Get(base + "/static/demo/merchants/MER-DEMO-0001-logo.png")
	if err != nil {
		t.Fatalf("GET demo asset: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "image/png" {
		t.Errorf("Content-Type = %q, want image/png", ct)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if len(body) < 8 || string(body[1:4]) != "PNG" {
		t.Errorf("body is not a PNG: % x", body[:min(8, len(body))])
	}
}

// TestProductionRefusesToStartWithoutAnSMSGateway.
//
// This replaced an earlier test that started a production server to check demo
// assets were absent. It could no longer start one — because P04 made the
// development SMS sender refuse to be constructed in production, which is the
// stronger guarantee: a production deployment with no gateway would otherwise
// accept sign-ins and write every one-time code to the application log, where
// far more people can read it than should ever be able to sign in as a
// customer. The demo-asset rule is asserted in the unit tests instead.
func TestProductionRefusesToStartWithoutAnSMSGateway(t *testing.T) {
	cmd := exec.Command(buildAPI(t))
	cmd.Env = []string{
		"API_ADDR=127.0.0.1:0",
		"APP_ENV=production",
		"PUBLIC_BASE_URL=https://api.example.com",
		"JWT_SIGNING_KEY=a-real-production-signing-key-at-least-32-chars",
		"PAYMENT_WEBHOOK_SECRET=a-real-production-webhook-secret-at-least-32-chars",
		"DATABASE_URL=postgres://delivery:delivery@127.0.0.1:5432/delivery?sslmode=disable",
		"REDIS_URL=redis://127.0.0.1:6379/0",
		"PATH=" + os.Getenv("PATH"),
	}
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("a production server started with no SMS gateway configured:\n%s", out)
	}
	if !strings.Contains(string(out), "SMS") && !strings.Contains(string(out), "sms") {
		t.Errorf("the refusal does not mention SMS, so an operator cannot tell what to fix:\n%s", out)
	}
}

// TestMissingRequiredConfigStopsStartup: a server that starts without a
// database and fails later is harder to diagnose than one that never starts.
func TestMissingRequiredConfigStopsStartup(t *testing.T) {
	cmd := exec.Command(buildAPI(t))
	cmd.Env = []string{"API_ADDR=127.0.0.1:0", "PATH=" + os.Getenv("PATH")}
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("api started with no DATABASE_URL or REDIS_URL:\n%s", out)
	}
	for _, want := range []string{"DATABASE_URL", "REDIS_URL"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("startup error does not name %s:\n%s", want, out)
		}
	}
}

// TestGracefulShutdown asserts SIGTERM stops the server without a hard kill,
// which is what lets a rolling deploy drain in-flight requests.
func TestGracefulShutdown(t *testing.T) {
	base, stop := startAPI(t, buildAPI(t))

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(base + "/healthz")
	if err != nil {
		t.Fatalf("pre-shutdown request: %v", err)
	}
	_ = resp.Body.Close()

	stop() // asserts a clean exit

	if _, err := client.Get(base + "/healthz"); err == nil {
		t.Error("server still answering after shutdown")
	}
}

// TestReadinessIsTheProbeThatTouchesTheStores.
//
// The two probes answer different questions and a runbook that confuses them
// causes an outage rather than preventing one: a failed liveness check
// restarts the process, and restarting it because Postgres is unreachable
// fixes nothing while taking the healthy replicas down with it. So
// `/healthz` must stay dependency-free and `/readyz` must not.
//
// Both halves are asserted here, because a readiness probe that cannot go red
// is a readiness probe in name only.
func TestReadinessIsTheProbeThatTouchesTheStores(t *testing.T) {
	bin := buildAPI(t)
	client := &http.Client{Timeout: 5 * time.Second}

	// startAPI's own defaults point at the compose ports, which are not
	// necessarily where this machine's Postgres is. The probe under test is
	// about reachability, so the URLs it is given have to be the real ones.
	goodDB := "DATABASE_URL=" + dbtestSchemaURL(t)
	goodRedis := "REDIS_URL=" + os.Getenv("REDIS_URL")

	read := func(t *testing.T, url string) (int, map[string]string) {
		t.Helper()
		resp, err := client.Get(url) //nolint:noctx // fixed test-local URL
		if err != nil {
			t.Fatalf("GET %s: %v", url, err)
		}
		defer func() { _ = resp.Body.Close() }()
		var body map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("decode %s: %v", url, err)
		}
		return resp.StatusCode, body
	}

	t.Run("ready when both stores answer", func(t *testing.T) {
		base, stop := startAPI(t, bin, goodDB, goodRedis)
		defer stop()

		if status, body := read(t, base+"/readyz"); status != http.StatusOK ||
			body["status"] != "ready" {
			t.Errorf("readyz = %d %v, want 200 ready", status, body)
		}
	})

	// The process does not connect to either store at startup, which is what
	// makes these two cases testable at all: it comes up, serves /healthz, and
	// only the readiness probe notices that nothing is behind it.
	t.Run("unready, and says which dependency, when the database is gone", func(t *testing.T) {
		base, stop := startAPI(t, bin, goodRedis,
			"DATABASE_URL=postgres://delivery@127.0.0.1:1/nowhere?sslmode=disable")
		defer stop()

		if status, body := read(t, base+"/healthz"); status != http.StatusOK {
			t.Errorf("healthz = %d %v; liveness must not depend on Postgres", status, body)
		}
		status, body := read(t, base+"/readyz")
		if status != http.StatusServiceUnavailable {
			t.Errorf("readyz = %d, want 503", status)
		}
		if body["dependency"] != "database" {
			t.Errorf("readyz names %q; an operator cannot tell what to fix", body["dependency"])
		}
	})

	t.Run("unready, and says which dependency, when redis is gone", func(t *testing.T) {
		base, stop := startAPI(t, bin, goodDB, "REDIS_URL=redis://127.0.0.1:1/0")
		defer stop()

		if status, body := read(t, base+"/healthz"); status != http.StatusOK {
			t.Errorf("healthz = %d %v; liveness must not depend on Redis", status, body)
		}
		status, body := read(t, base+"/readyz")
		if status != http.StatusServiceUnavailable {
			t.Errorf("readyz = %d, want 503", status)
		}
		if body["dependency"] != "redis" {
			t.Errorf("readyz names %q; an operator cannot tell what to fix", body["dependency"])
		}
	})
}
