// Package openapi checks that api/openapi.yaml describes the API the server
// actually serves.
//
// A spec is only worth having if it is true. The Flutter client is generated
// from it (P17), so a served route that is undocumented is a route no client
// can call, and a documented route nothing serves is a client method that 404s
// in production. These tests make both states fail the build.
package openapi

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	geohttp "github.com/rootlogic-lab/delivery/backend/internal/modules/geo/transport/http"
)

type spec struct {
	Paths      map[string]map[string]operation `yaml:"paths"`
	Components struct {
		Schemas map[string]any `yaml:"schemas"`
	} `yaml:"components"`
}

type operation struct {
	Summary    string         `yaml:"summary"`
	Responses  map[string]any `yaml:"responses"`
	Parameters []struct {
		Name     string `yaml:"name"`
		In       string `yaml:"in"`
		Required bool   `yaml:"required"`
	} `yaml:"parameters"`
}

func loadSpec(t *testing.T) spec {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "api", "openapi.yaml"))
	if err != nil {
		t.Fatalf("read openapi.yaml: %v", err)
	}
	var s spec
	if err := yaml.Unmarshal(raw, &s); err != nil {
		t.Fatalf("parse openapi.yaml: %v", err)
	}
	if len(s.Paths) == 0 {
		t.Fatal("openapi.yaml declares no paths; the spec did not parse as expected")
	}
	return s
}

// servedRoutes lists every route the API mounts, as "METHOD /path".
//
// Each module contributes its own Patterns(), which is the same list its
// Register mounts — not a second copy maintained by hand. A new module must be
// added here; once added, its routes are checked forever.
func servedRoutes() []string {
	routes := []string{"GET /healthz"}
	routes = append(routes, geohttp.Patterns()...)
	sort.Strings(routes)
	return routes
}

var methodPath = regexp.MustCompile(`^([A-Z]+)\s+(\S+)$`)

func split(t *testing.T, route string) (method, path string) {
	t.Helper()
	m := methodPath.FindStringSubmatch(route)
	if m == nil {
		t.Fatalf("route %q is not %q", route, "METHOD /path")
	}
	return strings.ToLower(m[1]), m[2]
}

// TestEveryServedRouteIsDocumented: an undocumented route is one no generated
// client can reach.
func TestEveryServedRouteIsDocumented(t *testing.T) {
	s := loadSpec(t)
	for _, route := range servedRoutes() {
		method, path := split(t, route)
		ops, ok := s.Paths[path]
		if !ok {
			t.Errorf("%s is served but %s is absent from api/openapi.yaml", route, path)
			continue
		}
		if _, ok := ops[method]; !ok {
			t.Errorf("%s is served but openapi.yaml documents no %s for it", route, strings.ToUpper(method))
		}
	}
}

// TestEveryDocumentedRouteIsServed: a documented route nothing serves becomes a
// generated client method that fails at runtime.
func TestEveryDocumentedRouteIsServed(t *testing.T) {
	served := map[string]bool{}
	for _, route := range servedRoutes() {
		method, path := split(t, route)
		served[method+" "+path] = true
	}
	for path, ops := range loadSpec(t).Paths {
		for method := range ops {
			if !served[method+" "+path] {
				t.Errorf("openapi.yaml documents %s %s, which nothing serves",
					strings.ToUpper(method), path)
			}
		}
	}
}

// TestEveryOperationDocumentsItsFailures: a spec listing only the happy path
// generates clients that treat every error as a surprise.
func TestEveryOperationDocumentsItsFailures(t *testing.T) {
	for path, ops := range loadSpec(t).Paths {
		if path == "/healthz" {
			continue // a liveness probe that can fail is not a liveness probe
		}
		for method, op := range ops {
			if len(op.Responses) < 2 {
				t.Errorf("%s %s documents %d response(s); every operation must document its failures too",
					strings.ToUpper(method), path, len(op.Responses))
			}
			if strings.TrimSpace(op.Summary) == "" {
				t.Errorf("%s %s has no summary", strings.ToUpper(method), path)
			}
		}
	}
}

// TestRequiredParametersAreReallyRequired checks the spec against the running
// handler: a parameter documented as required must actually be rejected when
// it is absent, or the documentation is a promise the server does not keep.
func TestRequiredParametersAreReallyRequired(t *testing.T) {
	mux := http.NewServeMux()
	geohttp.NewHandler(nil).Register(mux)

	checked := 0
	for path, ops := range loadSpec(t).Paths {
		op, ok := ops["get"]
		if !ok || path == "/healthz" {
			continue
		}
		for _, param := range op.Parameters {
			if !param.Required || param.In != "query" {
				continue
			}
			// Send every other required parameter, omitting just this one.
			query := make([]string, 0, len(op.Parameters))
			for _, other := range op.Parameters {
				if other.Name == param.Name || !other.Required || other.In != "query" {
					continue
				}
				query = append(query, other.Name+"=1")
			}
			target := path + "?" + strings.Join(query, "&")

			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
			if rec.Code != http.StatusBadRequest {
				t.Errorf("%s: omitting the required parameter %q returned %d, want 400",
					path, param.Name, rec.Code)
			}
			checked++
		}
	}
	if checked == 0 {
		t.Error("no required query parameters were checked; this test is not doing anything")
	}
}

// TestErrorEnvelopeIsDefinedOnce: one error shape across the API means a client
// has one error type, not one per endpoint.
func TestErrorEnvelopeIsDefinedOnce(t *testing.T) {
	s := loadSpec(t)
	for _, required := range []string{"Error", "ErrorResponse", "Money", "Capability"} {
		if _, ok := s.Components.Schemas[required]; !ok {
			t.Errorf("components.schemas.%s is missing", required)
		}
	}
}
