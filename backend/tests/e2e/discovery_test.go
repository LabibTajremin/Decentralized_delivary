package e2e

import (
	"fmt"
	"net/http"
	"testing"
)

// These drive the decentralization model through the real binary against real
// PostGIS and the real seeded geography. The unit tests prove the ladder
// arithmetic; only this level proves that D1, D2 and D3 hold over geometry
// somebody actually drew — which is where the last such bug was found.

type discoveryExpansion struct {
	Level       int     `json:"level"`
	RadiusM     float64 `json:"radiusM"`
	Radius      string  `json:"radius"`
	Stage       string  `json:"stage"`
	CanExpand   bool    `json:"canExpand"`
	Offered     bool    `json:"offered"`
	AtCeiling   bool    `json:"atCeiling"`
	NextRadiusM float64 `json:"nextRadiusM"`
}

type discoveryMerchant struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Type      string  `json:"type"`
	AreaName  string  `json:"areaName"`
	DistanceM float64 `json:"distanceM"`
	Distance  string  `json:"distance"`
	Delivery  struct {
		Minor    int64  `json:"minor"`
		Currency string `json:"currency"`
		Display  string `json:"display"`
		Expanded bool   `json:"expanded"`
	} `json:"delivery"`
}

type discoverySearch struct {
	Division  string              `json:"division"`
	AreaName  string              `json:"areaName"`
	Notice    string              `json:"notice"`
	Total     int                 `json:"total"`
	Expansion discoveryExpansion  `json:"expansion"`
	Merchants []discoveryMerchant `json:"merchants"`
}

// discover runs one search against the live API.
func discover(t *testing.T, base string, lat, lng float64, query string) discoverySearch {
	t.Helper()
	url := fmt.Sprintf("%s/v1/discovery/merchants?lat=%f&lng=%f", base, lat, lng)
	if query != "" {
		url += "&" + query
	}
	var got discoverySearch
	if status := getJSON(t, url, &got); status != http.StatusOK {
		t.Fatalf("GET %s: status = %d", url, status)
	}
	return got
}

// The seeded geography these tests stand on. Dhanmondi and Agrabad are in
// different divisions and 240 km apart, which is what makes the D3 assertion
// below meaningful rather than a coincidence of the radius.
const (
	dhanmondiLat, dhanmondiLng = 23.7455, 90.3738
	agrabadLat, agrabadLng     = 22.3284, 91.8118
	// A point in Rangpur division, where the seed places no shops at all.
	emptyLat, emptyLng = 25.7439, 89.2752
)

// D1: a customer sees the shops near them, with every string already composed.
func TestACustomerSeesWhatIsNearThem(t *testing.T) {
	base, stop := startAPIWithDB(t)
	defer stop()

	got := discover(t, base, dhanmondiLat, dhanmondiLng, "lang=en")

	if got.Division != "Dhaka" {
		t.Fatalf("division = %q, want Dhaka", got.Division)
	}
	if got.Total == 0 {
		t.Fatal("Dhanmondi returned no shops; the seed places several within 5 km")
	}
	if got.Expansion.Stage != "base" || got.Expansion.RadiusM != 5000 {
		t.Errorf("expansion = %+v, want the 5 km base radius", got.Expansion)
	}

	// Every card final: no distance to round, no fee to compute, no open-status
	// sentence to assemble (2.9). The order is the ranker's, not distance's —
	// an open shop outranks a nearer closed one (ALG-07) — so what is asserted
	// here is the radius bound, not the sequence.
	for _, m := range got.Merchants {
		if m.DistanceM > 5000 {
			t.Errorf("%s is %.0f m away, outside the 5 km radius", m.ID, m.DistanceM)
		}
		if m.Distance == "" || m.Delivery.Display == "" || m.Delivery.Currency != "BDT" {
			t.Errorf("%s is not fully composed: %+v", m.ID, m)
		}
		if m.Delivery.Expanded {
			t.Errorf("%s carries an expansion surcharge at the base radius", m.ID)
		}
	}
}

// D3, the acceptance criterion stated directly: two customers in two divisions
// see two disjoint sets, at every expansion level.
func TestTwoDivisionsNeverSeeEachOthersMerchants(t *testing.T) {
	base, stop := startAPIWithDB(t)
	defer stop()

	dhaka := discover(t, base, dhanmondiLat, dhanmondiLng, "level=4&limit=50")
	chattogram := discover(t, base, agrabadLat, agrabadLng, "level=4&limit=50")

	if dhaka.Division != "Dhaka" || chattogram.Division != "Chattogram" {
		t.Fatalf("divisions resolved to %q and %q", dhaka.Division, chattogram.Division)
	}
	if dhaka.Total == 0 || chattogram.Total == 0 {
		t.Fatalf("one of the divisions is empty: %d and %d", dhaka.Total, chattogram.Total)
	}

	seen := map[string]bool{}
	for _, m := range dhaka.Merchants {
		seen[m.ID] = true
	}
	for _, m := range chattogram.Merchants {
		if seen[m.ID] {
			t.Errorf("%s (%s) is visible from both divisions", m.ID, m.Name)
		}
	}

	// Both are at the widest radius the ladder allows and both report it as
	// terminal. That is what makes the disjointness a property of D3 rather
	// than of the two points happening to be far apart.
	for name, got := range map[string]discoverySearch{"Dhaka": dhaka, "Chattogram": chattogram} {
		if !got.Expansion.AtCeiling || got.Expansion.CanExpand {
			t.Errorf("%s at level 4: %+v, want a terminal ceiling", name, got.Expansion)
		}
	}
}

// D2: nothing nearby, so a wider search is offered — and the notice says it
// will cost more, because it will.
func TestNothingNearbyOffersAWiderSearch(t *testing.T) {
	base, stop := startAPIWithDB(t)
	defer stop()

	got := discover(t, base, emptyLat, emptyLng, "lang=en")
	if got.Total != 0 {
		t.Fatalf("a point with no seeded shops returned %d", got.Total)
	}
	if !got.Expansion.CanExpand || !got.Expansion.Offered {
		t.Fatalf("no expansion was offered: %+v", got.Expansion)
	}
	if got.Expansion.NextRadiusM != 10000 {
		t.Errorf("next radius = %v, want 10000", got.Expansion.NextRadiusM)
	}
	if want := "No shops within 5 km. Try searching up to 10 km — delivery will cost more."; got.Notice != want {
		t.Errorf("notice = %q, want %q", got.Notice, want)
	}
}

// D2, the money half: the same shop, quoted at two levels.
func TestExpandingRaisesTheQuotedDeliveryFee(t *testing.T) {
	base, stop := startAPIWithDB(t)
	defer stop()

	atBase := discover(t, base, dhanmondiLat, dhanmondiLng, "limit=50")
	expanded := discover(t, base, dhanmondiLat, dhanmondiLng, "level=1&limit=50")

	fees := map[string]int64{}
	for _, m := range atBase.Merchants {
		fees[m.ID] = m.Delivery.Minor
	}

	compared := 0
	for _, m := range expanded.Merchants {
		before, ok := fees[m.ID]
		if !ok {
			continue // a shop only the wider radius reached
		}
		if m.Delivery.Minor <= before {
			t.Errorf("%s: fee %d at level 1 is not above %d at level 0", m.ID, m.Delivery.Minor, before)
		}
		if !m.Delivery.Expanded {
			t.Errorf("%s: the surcharge was applied without being flagged", m.ID)
		}
		compared++
	}
	if compared == 0 {
		t.Fatal("no shop appeared at both levels; this test compared nothing")
	}
}

// D3: expansion stops, and says so, rather than handing back a button that
// could only fail.
func TestExpansionStopsAtTheDivisionCeiling(t *testing.T) {
	base, stop := startAPIWithDB(t)
	defer stop()

	got := discover(t, base, emptyLat, emptyLng, "level=99&lang=en")
	if got.Expansion.Level != 4 || got.Expansion.RadiusM != 25000 {
		t.Fatalf("a level beyond the ladder was not clamped: %+v", got.Expansion)
	}
	if got.Expansion.Stage != "ceiling" || !got.Expansion.AtCeiling || got.Expansion.CanExpand {
		t.Fatalf("the ceiling was not terminal: %+v", got.Expansion)
	}
	if want := "No shops found within 25 km, which is as far as we can search in your division."; got.Notice != want {
		t.Errorf("terminal notice = %q, want %q", got.Notice, want)
	}
}

func TestSearchFiltersAndPagesOverRealData(t *testing.T) {
	base, stop := startAPIWithDB(t)
	defer stop()

	pharmacies := discover(t, base, dhanmondiLat, dhanmondiLng, "type=pharmacy&level=4&limit=50")
	for _, m := range pharmacies.Merchants {
		if m.Type != "pharmacy" {
			t.Errorf("%s is a %s in a pharmacy search", m.ID, m.Type)
		}
	}

	all := discover(t, base, dhanmondiLat, dhanmondiLng, "level=4&limit=50")
	if pharmacies.Total >= all.Total {
		t.Errorf("the type filter did not narrow anything: %d of %d", pharmacies.Total, all.Total)
	}

	firstPage := discover(t, base, dhanmondiLat, dhanmondiLng, "level=4&limit=2&offset=0")
	secondPage := discover(t, base, dhanmondiLat, dhanmondiLng, "level=4&limit=2&offset=2")
	if len(firstPage.Merchants) != 2 || len(secondPage.Merchants) != 2 {
		t.Fatalf("pages have %d and %d entries", len(firstPage.Merchants), len(secondPage.Merchants))
	}
	// Paging must not repeat a shop. It is the tie-break in the ranking that
	// makes this true, and it only shows up against real data.
	if firstPage.Merchants[0].ID == secondPage.Merchants[0].ID {
		t.Error("the second page repeated the first")
	}
	if firstPage.Total != all.Total {
		t.Errorf("total changed with the page size: %d then %d", firstPage.Total, all.Total)
	}
}

// The single-merchant answer cart will ask for, over the same geometry.
func TestReachabilityOverRealGeography(t *testing.T) {
	base, stop := startAPIWithDB(t)
	defer stop()

	near := discover(t, base, dhanmondiLat, dhanmondiLng, "limit=1")
	if near.Total == 0 {
		t.Fatal("no shop near Dhanmondi to ask about")
	}
	nearbyID := near.Merchants[0].ID

	type reachBody struct {
		MerchantID    string  `json:"merchantId"`
		DistanceM     float64 `json:"distanceM"`
		Distance      string  `json:"distance"`
		Reachable     bool    `json:"reachable"`
		RequiredLevel int     `json:"requiredLevel"`
		Expanded      bool    `json:"expanded"`
		Reason        string  `json:"reason"`
	}

	var fromHome reachBody
	url := fmt.Sprintf("%s/v1/discovery/merchants/%s?lat=%f&lng=%f&lang=en",
		base, nearbyID, dhanmondiLat, dhanmondiLng)
	if status := getJSON(t, url, &fromHome); status != http.StatusOK {
		t.Fatalf("reach: status = %d", status)
	}
	if !fromHome.Reachable || fromHome.RequiredLevel != 0 || fromHome.Expanded {
		t.Fatalf("a shop on the same road was not in the base radius: %+v", fromHome)
	}

	// The same shop, asked about from Chattogram. D3 makes it unreachable at
	// every level, which is a different answer from "too far for now".
	var fromAway reachBody
	url = fmt.Sprintf("%s/v1/discovery/merchants/%s?lat=%f&lng=%f",
		base, nearbyID, agrabadLat, agrabadLng)
	if status := getJSON(t, url, &fromAway); status != http.StatusOK {
		t.Fatalf("cross-division reach: status = %d", status)
	}
	if fromAway.Reachable || fromAway.Reason != "outside_division" {
		t.Fatalf("a Dhaka shop was reachable from Chattogram: %+v", fromAway)
	}
	// The distance is still reported — it is true — so the app can say how far
	// away it is while refusing the order.
	if fromAway.DistanceM < 200_000 {
		t.Errorf("distance = %v m, want the real 240 km", fromAway.DistanceM)
	}
}

// Discovery is public. A customer who has to create an account to find out
// whether anything delivers to their village is a customer who does not.
func TestDiscoveryNeedsNoAccount(t *testing.T) {
	base, stop := startAPIWithDB(t)
	defer stop()

	url := fmt.Sprintf("%s/v1/discovery/merchants?lat=%f&lng=%f", base, dhanmondiLat, dhanmondiLng)
	if status := getJSONAs(t, url, "", nil); status != http.StatusOK {
		t.Fatalf("an anonymous search returned %d", status)
	}
}

// A point in the sea belongs to no division, so there is no ladder to walk and
// nothing honest to show.
func TestAPointOutsideEveryDivisionIsRefused(t *testing.T) {
	base, stop := startAPIWithDB(t)
	defer stop()

	url := fmt.Sprintf("%s/v1/discovery/merchants?lat=%f&lng=%f", base, 19.0, 88.0)
	if status := getJSON(t, url, nil); status != http.StatusNotFound {
		t.Fatalf("a point in the Bay of Bengal returned %d, want 404", status)
	}
}
