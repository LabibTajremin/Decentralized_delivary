package confighttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/config/application"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/config/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/config/domain"
	confighttp "github.com/rootlogic-lab/delivery/backend/internal/modules/config/transport/http"
)

// memoryRepo is a minimal in-memory repository so these tests exercise the
// transport layer against real use cases rather than a stubbed interface —
// which is what catches a handler that parses a value into the wrong type.
type memoryRepo struct {
	overrides []domain.Override
	changes   []domain.Change
	loadErr   error
}

func (m *memoryRepo) OverridesFor(context.Context, domain.Placement) ([]domain.Override, error) {
	if m.loadErr != nil {
		return nil, m.loadErr
	}
	return m.overrides, nil
}

func (m *memoryRepo) AllOverrides(_ context.Context, scope domain.Scope) ([]domain.Override, error) {
	var out []domain.Override
	for _, o := range m.overrides {
		if o.Scope == scope {
			out = append(out, o)
		}
	}
	return out, nil
}

func (m *memoryRepo) SaveOverride(_ context.Context, o domain.Override, c domain.Change) error {
	for i, existing := range m.overrides {
		if existing.Key == o.Key && existing.Scope == o.Scope {
			m.overrides[i] = o
			m.changes = append(m.changes, c)
			return nil
		}
	}
	m.overrides = append(m.overrides, o)
	m.changes = append(m.changes, c)
	return nil
}

func (m *memoryRepo) DeleteOverride(_ context.Context, key domain.Key, scope domain.Scope, c domain.Change) error {
	kept := m.overrides[:0]
	for _, o := range m.overrides {
		if o.Key == key && o.Scope == scope {
			continue
		}
		kept = append(kept, o)
	}
	m.overrides = kept
	m.changes = append(m.changes, c)
	return nil
}

func (m *memoryRepo) Changes(context.Context, ports.ChangeFilter) ([]domain.Change, error) {
	return m.changes, nil
}

type fixedClock struct{}

func (fixedClock) Now() time.Time { return time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC) }

type fixedIDs struct{}

func (fixedIDs) New(prefix string) string { return prefix + "_fixed" }

func newMux(repo *memoryRepo) *http.ServeMux {
	mux := http.NewServeMux()
	confighttp.NewHandler(
		application.NewResolveUseCase(repo),
		application.NewSetOverrideUseCase(repo, fixedClock{}, fixedIDs{}),
		application.NewClearOverrideUseCase(repo, fixedClock{}, fixedIDs{}),
	).Register(mux)
	return mux
}

func do(mux *http.ServeMux, method, target, body string) *httptest.ResponseRecorder {
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
		t.Fatalf("decode: %v\nbody: %s", err, rec.Body.String())
	}
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	decode(t, rec, &body)
	return body.Error.Code
}

// TestDefinitionsListsEveryVariableWithItsBounds: an admin editing a value
// needs to know what is allowed before submitting, not after being rejected.
func TestDefinitionsListsEveryVariableWithItsBounds(t *testing.T) {
	rec := do(newMux(&memoryRepo{}), http.MethodGet, "/v1/config/definitions", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}

	var body struct {
		Definitions []struct {
			Key         string `json:"key"`
			Type        string `json:"type"`
			Unit        string `json:"unit"`
			Default     string `json:"default"`
			Minimum     string `json:"minimum"`
			Maximum     string `json:"maximum"`
			AutoTunable bool   `json:"auto_tunable"`
			Immutable   bool   `json:"immutable"`
			Purpose     string `json:"purpose"`
		} `json:"definitions"`
	}
	decode(t, rec, &body)

	if len(body.Definitions) != len(domain.AllKeys()) {
		t.Fatalf("listed %d definitions, registry has %d", len(body.Definitions), len(domain.AllKeys()))
	}
	for _, d := range body.Definitions {
		if d.Purpose == "" {
			t.Errorf("%s has no purpose", d.Key)
		}
		if d.Type != "bool" && (d.Minimum == "" || d.Maximum == "") {
			t.Errorf("%s has no bounds for an admin to work within", d.Key)
		}
		if d.Type == "bool" && (d.Minimum != "" || d.Maximum != "") {
			t.Errorf("%s is a bool but reports bounds %q..%q, which reads as forbidding true",
				d.Key, d.Minimum, d.Maximum)
		}
	}
}

// TestTheDivisionCeilingIsVisibleAndLocked: it is in the registry so it is
// auditable, not so it is adjustable, and the admin UI has to be able to say so.
func TestTheDivisionCeilingIsVisibleAndLocked(t *testing.T) {
	rec := do(newMux(&memoryRepo{}), http.MethodGet, "/v1/config/definitions", "")
	var body struct {
		Definitions []struct {
			Key       string `json:"key"`
			Immutable bool   `json:"immutable"`
		} `json:"definitions"`
	}
	decode(t, rec, &body)

	found := false
	for _, d := range body.Definitions {
		if d.Key == string(domain.DiscoveryDivisionCeil) {
			found = true
			if !d.Immutable {
				t.Error("the division ceiling is not reported as immutable")
			}
		}
	}
	if !found {
		t.Error("the division ceiling is not listed at all")
	}
}

func TestEffectiveReportsValuesAndWhereTheyCameFrom(t *testing.T) {
	scope, _ := domain.NewScope(domain.LevelArea, "DHK-DHM")
	value, _ := domain.Money(6000)
	repo := &memoryRepo{overrides: []domain.Override{
		{Key: domain.PricingDeliveryBase, Scope: scope, Value: value},
	}}

	rec := do(newMux(repo), http.MethodGet,
		"/v1/config/effective?area=DHK-DHM&district=DHK&division=DHA", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Values []struct {
			Key    string `json:"key"`
			Value  string `json:"value"`
			Source string `json:"source"`
		} `json:"values"`
	}
	decode(t, rec, &body)

	if len(body.Values) != len(domain.AllKeys()) {
		t.Fatalf("returned %d values, want every key", len(body.Values))
	}
	for _, v := range body.Values {
		switch v.Key {
		case string(domain.PricingDeliveryBase):
			if v.Value != "6000" || v.Source != "area:DHK-DHM" {
				t.Errorf("delivery base = %s from %s, want 6000 from the area", v.Value, v.Source)
			}
		case string(domain.DiscoveryBaseRadius):
			if v.Value != "5000" || v.Source != "global" {
				t.Errorf("base radius = %s from %s, want the global default", v.Value, v.Source)
			}
		}
	}
}

func TestEffectiveSurfacesALoadFailure(t *testing.T) {
	repo := &memoryRepo{loadErr: context.DeadlineExceeded}
	rec := do(newMux(repo), http.MethodGet, "/v1/config/effective?area=DHK-DHM", "")
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
}

func TestSettingAnOverrideRecordsAChange(t *testing.T) {
	repo := &memoryRepo{}
	rec := do(newMux(repo), http.MethodPut, "/v1/config/overrides", `{
		"key": "pricing.delivery_base",
		"level": "area",
		"code": "DHK-DHM",
		"value": "6000",
		"reason": "traffic",
		"actor_id": "adm_1"
	}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	var change struct {
		Key       string `json:"key"`
		Scope     string `json:"scope"`
		OldValue  string `json:"old_value"`
		NewValue  string `json:"new_value"`
		Actor     string `json:"actor"`
		ChangedAt string `json:"changed_at"`
	}
	decode(t, rec, &change)

	if change.Scope != "area:DHK-DHM" || change.NewValue != "6000" || change.OldValue != "4000" {
		t.Errorf("change = %+v", change)
	}
	if change.Actor != "admin:adm_1" {
		t.Errorf("actor = %q", change.Actor)
	}
	if change.ChangedAt != "2026-03-01T12:00:00Z" {
		t.Errorf("changed_at = %q", change.ChangedAt)
	}
	if len(repo.changes) != 1 {
		t.Errorf("audit entries = %d", len(repo.changes))
	}
}

// TestMoneyIsSentAsTextNotAFloat: a JSON number for money invites a client to
// parse it as a float and lose minor units.
func TestMoneyIsSentAsTextNotAFloat(t *testing.T) {
	repo := &memoryRepo{}
	rec := do(newMux(repo), http.MethodPut, "/v1/config/overrides", `{
		"key": "pricing.delivery_base", "level": "global", "code": "",
		"value": "6000", "reason": "r", "actor_id": "adm_1"
	}`)
	var raw map[string]json.RawMessage
	decode(t, rec, &raw)
	if string(raw["new_value"]) != `"6000"` {
		t.Errorf("new_value = %s, want a quoted string", raw["new_value"])
	}
}

// TestTheAPIRefusesToDisableTheDivisionCeiling over HTTP, end to end.
func TestTheAPIRefusesToDisableTheDivisionCeiling(t *testing.T) {
	repo := &memoryRepo{}
	rec := do(newMux(repo), http.MethodPut, "/v1/config/overrides", `{
		"key": "discovery.division_ceiling",
		"level": "area", "code": "DHK-DHM",
		"value": "false",
		"reason": "we want cross-division delivery",
		"actor_id": "adm_1"
	}`)
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
	if got := errorCode(t, rec); got != "immutable_config_key" {
		t.Errorf("code = %q", got)
	}
	if len(repo.overrides) != 0 {
		t.Error("a refused change wrote an override")
	}
}

func TestSettingOutOfBoundsIsRejected(t *testing.T) {
	rec := do(newMux(&memoryRepo{}), http.MethodPut, "/v1/config/overrides", `{
		"key": "pricing.delivery_base", "level": "global", "code": "",
		"value": "1000000", "reason": "surge", "actor_id": "adm_1"
	}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	if got := errorCode(t, rec); got != "config_out_of_bounds" {
		t.Errorf("code = %q", got)
	}
}

func TestSettingAValueOfTheWrongShapeIsRejected(t *testing.T) {
	rec := do(newMux(&memoryRepo{}), http.MethodPut, "/v1/config/overrides", `{
		"key": "pricing.delivery_base", "level": "global", "code": "",
		"value": "quite a lot", "reason": "r", "actor_id": "adm_1"
	}`)
	if got := errorCode(t, rec); got != "invalid_config_value" {
		t.Errorf("code = %q, want invalid_config_value", got)
	}
}

func TestSettingAnUnknownKeyIsRejected(t *testing.T) {
	rec := do(newMux(&memoryRepo{}), http.MethodPut, "/v1/config/overrides", `{
		"key": "pricing.delivery_bass", "level": "global", "code": "",
		"value": "6000", "reason": "r", "actor_id": "adm_1"
	}`)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
	if got := errorCode(t, rec); got != "unknown_config_key" {
		t.Errorf("code = %q", got)
	}
}

func TestAnInvalidScopeIsRejected(t *testing.T) {
	cases := []struct{ body, wantCode string }{
		{`{"key":"pricing.delivery_base","level":"planet","code":"x","value":"6000","reason":"r","actor_id":"a"}`, "invalid_scope_level"},
		{`{"key":"pricing.delivery_base","level":"area","code":"","value":"6000","reason":"r","actor_id":"a"}`, "invalid_scope"},
		{`{"key":"pricing.delivery_base","level":"global","code":"DHA","value":"6000","reason":"r","actor_id":"a"}`, "invalid_scope"},
	}
	for _, c := range cases {
		rec := do(newMux(&memoryRepo{}), http.MethodPut, "/v1/config/overrides", c.body)
		if got := errorCode(t, rec); got != c.wantCode {
			t.Errorf("body %s: code = %q, want %q", c.body, got, c.wantCode)
		}
	}
}

func TestAChangeWithoutAReasonIsRejectedOverHTTP(t *testing.T) {
	rec := do(newMux(&memoryRepo{}), http.MethodPut, "/v1/config/overrides", `{
		"key": "pricing.delivery_base", "level": "global", "code": "",
		"value": "6000", "reason": "", "actor_id": "adm_1"
	}`)
	if got := errorCode(t, rec); got != "reason_required" {
		t.Errorf("code = %q", got)
	}
}

func TestAMalformedBodyIsRejected(t *testing.T) {
	for _, body := range []string{`{"key":`, `{"unknown_field": 1}`} {
		rec := do(newMux(&memoryRepo{}), http.MethodPut, "/v1/config/overrides", body)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("body %s: status = %d, want 400", body, rec.Code)
		}
	}
}

func TestClearingAnOverrideOverHTTP(t *testing.T) {
	scope, _ := domain.NewScope(domain.LevelArea, "DHK-DHM")
	value, _ := domain.Money(6000)
	repo := &memoryRepo{overrides: []domain.Override{
		{Key: domain.PricingDeliveryBase, Scope: scope, Value: value},
	}}

	rec := do(newMux(repo), http.MethodDelete, "/v1/config/override", `{
		"key": "pricing.delivery_base", "level": "area", "code": "DHK-DHM",
		"reason": "trial over", "actor_id": "adm_1"
	}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if len(repo.overrides) != 0 {
		t.Errorf("overrides = %+v, want it removed", repo.overrides)
	}

	var change struct {
		OldValue string `json:"old_value"`
		NewValue string `json:"new_value"`
	}
	decode(t, rec, &change)
	if change.OldValue != "6000" || change.NewValue != "4000" {
		t.Errorf("change = %s -> %s", change.OldValue, change.NewValue)
	}
}

func TestClearingTheGlobalValueIsRejected(t *testing.T) {
	rec := do(newMux(&memoryRepo{}), http.MethodDelete, "/v1/config/override", `{
		"key": "pricing.delivery_base", "level": "global", "code": "",
		"reason": "tidying", "actor_id": "adm_1"
	}`)
	if got := errorCode(t, rec); got != "cannot_clear_global" {
		t.Errorf("code = %q", got)
	}
}

func TestClearingRejectsAnInvalidScopeAndBody(t *testing.T) {
	rec := do(newMux(&memoryRepo{}), http.MethodDelete, "/v1/config/override",
		`{"key":"pricing.delivery_base","level":"planet","code":"x","reason":"r","actor_id":"a"}`)
	if got := errorCode(t, rec); got != "invalid_scope_level" {
		t.Errorf("code = %q", got)
	}
	rec = do(newMux(&memoryRepo{}), http.MethodDelete, "/v1/config/override", `{"key":`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestPatternsMatchWhatRegisterMounts(t *testing.T) {
	mux := newMux(&memoryRepo{})
	for _, pattern := range confighttp.Patterns() {
		method, path, found := strings.Cut(pattern, " ")
		if !found {
			t.Fatalf("pattern %q is not %q", pattern, "METHOD /path")
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader("{}")))
		if rec.Code == http.StatusNotFound {
			t.Errorf("%s is in Patterns but nothing is mounted for it", pattern)
		}
	}
}
