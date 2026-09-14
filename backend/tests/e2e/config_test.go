package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
)

// These drive the config endpoints through the real binary against real
// PostgreSQL: HTTP in, resolution, transactional write, audit entry, JSON out.

func putJSON(t *testing.T, url, body string, into any) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewBufferString(body)) //nolint:noctx // fixed test-local URL
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT %s: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if into != nil {
		if err := json.NewDecoder(resp.Body).Decode(into); err != nil {
			t.Fatalf("decode: %v", err)
		}
	}
	return resp.StatusCode
}

func deleteJSON(t *testing.T, url, body string, into any) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, url, bytes.NewBufferString(body)) //nolint:noctx // fixed test-local URL
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE %s: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if into != nil {
		if err := json.NewDecoder(resp.Body).Decode(into); err != nil {
			t.Fatalf("decode: %v", err)
		}
	}
	return resp.StatusCode
}

// effectiveValue reads one setting out of the effective-config response.
func effectiveValue(t *testing.T, base, query, key string) (value, source string) {
	t.Helper()
	var body struct {
		Values []struct {
			Key    string `json:"key"`
			Value  string `json:"value"`
			Source string `json:"source"`
		} `json:"values"`
	}
	if status := getJSON(t, base+"/v1/config/effective?"+query, &body); status != http.StatusOK {
		t.Fatalf("effective config status = %d", status)
	}
	for _, v := range body.Values {
		if v.Key == key {
			return v.Value, v.Source
		}
	}
	t.Fatalf("%s is not in the effective configuration", key)
	return "", ""
}

// TestAnAreaOverrideTakesEffectEndToEnd is the whole point of the module:
// an admin changes a fee for one area and only that area sees it.
func TestAnAreaOverrideTakesEffectEndToEnd(t *testing.T) {
	base, stop := startAPIWithDB(t)
	defer stop()

	const dhanmondi = "area=DHK-DHM&district=DHK&division=DHA"
	const gulshan = "area=DHK-GUL&district=DHK&division=DHA"

	before, source := effectiveValue(t, base, dhanmondi, "pricing.delivery_base")
	if before != "4000" || source != "global" {
		t.Fatalf("starting value = %s from %s, want the 4000 default", before, source)
	}

	var change struct {
		OldValue string `json:"old_value"`
		NewValue string `json:"new_value"`
		Scope    string `json:"scope"`
		Reason   string `json:"reason"`
	}
	status := putJSON(t, base+"/v1/config/overrides", `{
		"key": "pricing.delivery_base",
		"level": "area", "code": "DHK-DHM",
		"value": "6000",
		"reason": "Dhanmondi traffic has got worse",
		"actor_id": "adm_e2e"
	}`, &change)
	if status != http.StatusOK {
		t.Fatalf("set override status = %d", status)
	}
	if change.OldValue != "4000" || change.NewValue != "6000" || change.Scope != "area:DHK-DHM" {
		t.Errorf("change = %+v", change)
	}

	after, source := effectiveValue(t, base, dhanmondi, "pricing.delivery_base")
	if after != "6000" || source != "area:DHK-DHM" {
		t.Errorf("Dhanmondi = %s from %s, want 6000 from the area override", after, source)
	}

	neighbour, source := effectiveValue(t, base, gulshan, "pricing.delivery_base")
	if neighbour != "4000" || source != "global" {
		t.Errorf("Gulshan = %s from %s; one area's override must not reach another", neighbour, source)
	}

	// And clearing it puts Dhanmondi back.
	if status := deleteJSON(t, base+"/v1/config/override", `{
		"key": "pricing.delivery_base",
		"level": "area", "code": "DHK-DHM",
		"reason": "trial over",
		"actor_id": "adm_e2e"
	}`, nil); status != http.StatusOK {
		t.Fatalf("clear override status = %d", status)
	}
	restored, source := effectiveValue(t, base, dhanmondi, "pricing.delivery_base")
	if restored != "4000" || source != "global" {
		t.Errorf("after clearing = %s from %s, want the default back", restored, source)
	}
}

// TestResolutionOrderOverHTTP: a district override applies to its areas, and an
// area override beats it.
func TestResolutionOrderOverHTTP(t *testing.T) {
	base, stop := startAPIWithDB(t)
	defer stop()

	const query = "area=DHK-MIR&district=DHK&division=DHA"

	for _, step := range []struct {
		level, code, value, wantSource string
	}{
		{"division", "DHA", "9000", "division:DHA"},
		{"district", "DHK", "8000", "district:DHK"},
		{"area", "DHK-MIR", "7000", "area:DHK-MIR"},
	} {
		body := `{"key":"discovery.base_radius","level":"` + step.level +
			`","code":"` + step.code + `","value":"` + step.value +
			`","reason":"resolution test","actor_id":"adm_e2e"}`
		if status := putJSON(t, base+"/v1/config/overrides", body, nil); status != http.StatusOK {
			t.Fatalf("set %s override: status %d", step.level, status)
		}
		value, source := effectiveValue(t, base, query, "discovery.base_radius")
		if value != step.value || source != step.wantSource {
			t.Errorf("after setting %s: value = %s from %s, want %s from %s",
				step.level, value, source, step.value, step.wantSource)
		}
	}
}

// TestTheDivisionCeilingCannotBeTurnedOffOverHTTP is D3 at the API boundary.
func TestTheDivisionCeilingCannotBeTurnedOffOverHTTP(t *testing.T) {
	base, stop := startAPIWithDB(t)
	defer stop()

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	status := putJSON(t, base+"/v1/config/overrides", `{
		"key": "discovery.division_ceiling",
		"level": "area", "code": "DHK-DHM",
		"value": "false",
		"reason": "we want cross-division delivery",
		"actor_id": "adm_e2e"
	}`, &body)

	if status != http.StatusForbidden {
		t.Errorf("status = %d, want 403", status)
	}
	if body.Error.Code != "immutable_config_key" {
		t.Errorf("code = %q", body.Error.Code)
	}

	value, _ := effectiveValue(t, base, "area=DHK-DHM", "discovery.division_ceiling")
	if value != "true" {
		t.Errorf("division ceiling = %s after a refused change, want true", value)
	}
}

func TestOutOfBoundsIsRejectedOverHTTP(t *testing.T) {
	base, stop := startAPIWithDB(t)
	defer stop()

	var body struct {
		Error struct {
			Code    string            `json:"code"`
			Details map[string]string `json:"details"`
		} `json:"error"`
	}
	status := putJSON(t, base+"/v1/config/overrides", `{
		"key": "pricing.delivery_base",
		"level": "global", "code": "",
		"value": "1000000",
		"reason": "surge",
		"actor_id": "adm_e2e"
	}`, &body)

	if status != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", status)
	}
	if body.Error.Details["maximum"] == "" {
		t.Errorf("the error must state the maximum, got %+v", body.Error.Details)
	}
}

func TestDefinitionsAreServed(t *testing.T) {
	base, stop := startAPIWithDB(t)
	defer stop()

	var body struct {
		Definitions []struct {
			Key       string `json:"key"`
			Immutable bool   `json:"immutable"`
		} `json:"definitions"`
	}
	if status := getJSON(t, base+"/v1/config/definitions", &body); status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	if len(body.Definitions) != 17 {
		t.Errorf("definitions = %d, want the 17 Appendix B variables", len(body.Definitions))
	}
}
