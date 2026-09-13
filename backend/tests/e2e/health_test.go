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
	port := freePort(t)
	cmd := exec.Command(bin)
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
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
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

// TestDemoAssetsAreAbsentInProduction is the same guard from the other side:
// a production binary must not serve placeholder merchant logos at all.
func TestDemoAssetsAreAbsentInProduction(t *testing.T) {
	base, stop := startAPI(t, buildAPI(t),
		"APP_ENV=production",
		"PUBLIC_BASE_URL=https://api.example.com",
		"JWT_SIGNING_KEY=a-real-production-signing-key-at-least-32-chars",
	)
	defer stop()

	resp, err := http.Get(base + "/static/demo/merchants/MER-DEMO-0001-logo.png")
	if err != nil {
		t.Fatalf("GET demo asset: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404: a production server must not serve demo imagery", resp.StatusCode)
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
