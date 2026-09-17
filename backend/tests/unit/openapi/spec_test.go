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

	carthttp "github.com/rootlogic-lab/delivery/backend/internal/modules/cart/transport/http"
	cathttp "github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/transport/http"
	confighttp "github.com/rootlogic-lab/delivery/backend/internal/modules/config/transport/http"
	discohttp "github.com/rootlogic-lab/delivery/backend/internal/modules/discovery/transport/http"
	dispatchhttp "github.com/rootlogic-lab/delivery/backend/internal/modules/dispatch/transport/http"
	geohttp "github.com/rootlogic-lab/delivery/backend/internal/modules/geo/transport/http"
	identityhttp "github.com/rootlogic-lab/delivery/backend/internal/modules/identity/transport/http"
	merchanthttp "github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/transport/http"
	orderhttp "github.com/rootlogic-lab/delivery/backend/internal/modules/order/transport/http"
	paymenthttp "github.com/rootlogic-lab/delivery/backend/internal/modules/payment/transport/http"
	userhttp "github.com/rootlogic-lab/delivery/backend/internal/modules/user/transport/http"
)

type spec struct {
	Paths      map[string]map[string]operation `yaml:"paths"`
	Components struct {
		Schemas map[string]any `yaml:"schemas"`
	} `yaml:"components"`
}

type operation struct {
	Summary string `yaml:"summary"`
	// Security is a pointer so "absent" and "explicitly empty" stay distinct:
	// an empty list declares an endpoint public, while absence inherits the
	// document's default of requiring a bearer token.
	Security   *[]map[string]any `yaml:"security"`
	Responses  map[string]any    `yaml:"responses"`
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
	routes = append(routes, confighttp.Patterns()...)
	routes = append(routes, identityhttp.Patterns()...)
	routes = append(routes, userhttp.Patterns()...)
	routes = append(routes, merchanthttp.Patterns()...)
	routes = append(routes, cathttp.Patterns()...)
	routes = append(routes, discohttp.Patterns()...)
	routes = append(routes, carthttp.Patterns()...)
	routes = append(routes, orderhttp.Patterns()...)
	routes = append(routes, dispatchhttp.Patterns()...)
	routes = append(routes, paymenthttp.Patterns()...)
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

// TestEveryProtectedOperationDeclaresItsAuth: an endpoint whose spec omits
// security reads as public to a generated client, which would then omit the
// Authorization header and get a 401 it has no handling for.
func TestPublicOperationsAreExplicitlyMarked(t *testing.T) {
	// The only endpoints that can work before a token exists. Rule 2.7 forbids
	// an implicitly-public endpoint, so this list is the whole set and a new
	// entry appearing here is a decision worth noticing.
	publicPaths := map[string]bool{
		"/healthz":             true,
		"/v1/geo/resolve":      true,
		"/v1/geo/merchants":    true,
		"/v1/geo/distance":     true,
		"/v1/auth/otp/request": true,
		"/v1/auth/otp/verify":  true,
		"/v1/auth/refresh":     true,

		// Browsing a shop's menu needs no account. The app shows a menu before
		// anyone signs in, and requiring a token to read one would put a
		// sign-up wall in front of the thing customers came for. Nothing here
		// exposes a shelf count, a visibility switch or an owner's details —
		// see the split between PublicItem and Item in the spec.
		"/v1/catalogue/{merchantId}/menu":             true,
		"/v1/catalogue/{merchantId}/items/{itemId}":   true,
		"/v1/catalogue/{merchantId}/combos/{comboId}": true,

		// Discovery is public for the same reason, and more sharply: a customer
		// who has to create an account to find out whether anything delivers to
		// their village is a customer who does not create an account. The
		// delivery point comes from the query rather than a saved address, so
		// nothing here reads anyone's data.
		"/v1/discovery/merchants":              true,
		"/v1/discovery/merchants/{merchantId}": true,

		// The one route in the whole API a person never calls. A gateway's own
		// servers reach it, authenticated by the payload's own signature
		// (WebhookUseCase.Handle) rather than a bearer token — the caller has
		// no account to hold one.
		"/v1/payments/manual/webhook": true,
	}
	for path, ops := range loadSpec(t).Paths {
		for method, op := range ops {
			declaredPublic := op.Security != nil && len(*op.Security) == 0
			if publicPaths[path] && !declaredPublic {
				t.Errorf("%s %s is public but does not declare `security: []`",
					strings.ToUpper(method), path)
			}
			if !publicPaths[path] && declaredPublic {
				t.Errorf("%s %s declares itself public but is not in the public list",
					strings.ToUpper(method), path)
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
	// Discovery reads its required parameters before it touches a use case, so
	// nil dependencies are enough to check that it refuses a missing one.
	discohttp.NewHandler(nil, nil).Register(mux)

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
