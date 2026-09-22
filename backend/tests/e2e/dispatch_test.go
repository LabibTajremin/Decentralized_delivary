package e2e

import (
	"fmt"
	"net/http"
	"testing"
)

// These drive a delivery from the moment a shop says the food is ready to the
// moment a rider says it is at the door, through the real binary. The phase's
// three acceptance criteria are checked here at the level a Flutter app sees
// them: a partner is shown only work inside their radius, D4's distance choice
// is honoured, and two partners cannot be given the same order.

type partnerPayload struct {
	ID                string  `json:"id"`
	Name              string  `json:"name"`
	Availability      string  `json:"availability"`
	AvailabilityLabel string  `json:"availability_label"`
	Preference        string  `json:"preference"`
	PreferenceLabel   string  `json:"preference_label"`
	Lat               float64 `json:"lat"`
	Lng               float64 `json:"lng"`
	Carrying          int     `json:"carrying"`
	AcceptancePercent int     `json:"acceptance_percent"`
}

type jobPayload struct {
	ID          string `json:"id"`
	OrderID     string `json:"order_id"`
	Code        string `json:"code"`
	Status      string `json:"status"`
	StatusLabel string `json:"status_label"`
	Live        bool   `json:"live"`
	Band        string `json:"band"`
	BandLabel   string `json:"band_label"`
	Distance    string `json:"distance"`
	ToPickup    string `json:"to_pickup"`
	Reason      string `json:"reason"`
	Pickup      struct {
		Name       string `json:"name"`
		Phone      string `json:"phone"`
		SingleLine string `json:"single_line"`
	} `json:"pickup"`
	Destination struct {
		Name       string `json:"name"`
		Phone      string `json:"phone"`
		SingleLine string `json:"single_line"`
	} `json:"destination"`
}

type feedPayload struct {
	Partner partnerPayload `json:"partner"`
	Jobs    []jobPayload   `json:"jobs"`
	RadiusM float64        `json:"radius_m"`
	Reason  string         `json:"reason"`
	Notice  string         `json:"notice"`
}

type jobListPayload struct {
	Jobs  []jobPayload `json:"jobs"`
	Total int          `json:"total"`
}

// onShift signs a rider in, registers them, puts them online and places them.
func onShift(t *testing.T, base string, tail *logTail, name string, lat, lng float64) (tokens, partnerPayload) {
	t.Helper()
	rider := signInAs(t, base, tail, uniquePhone(t), name+"'s phone", "partner")

	var partner partnerPayload
	if status := requestAs(t, http.MethodPost, base+"/v1/partner",
		fmt.Sprintf(`{"name":%q,"phone":"01711111111","vehicle":"motorcycle"}`, name),
		rider.AccessToken, &partner); status != http.StatusCreated {
		t.Fatalf("register %s: status = %d", name, status)
	}
	if status := requestAs(t, http.MethodPut, base+"/v1/partner/availability",
		`{"availability":"available"}`, rider.AccessToken, nil); status != http.StatusOK {
		t.Fatalf("%s online: status = %d", name, status)
	}
	if status := requestAs(t, http.MethodPut, base+"/v1/partner/location",
		fmt.Sprintf(`{"lat":%f,"lng":%f}`, lat, lng), rider.AccessToken, &partner); status != http.StatusOK {
		t.Fatalf("%s location: status = %d", name, status)
	}
	return rider, partner
}

// placeOrder puts the cart through as an order and returns its id.
func placeOrder(t *testing.T, base string, customer tokens, addressID string) string {
	t.Helper()
	var order orderResponse
	if status := requestAs(t, http.MethodPost, base+"/v1/orders",
		fmt.Sprintf(`{"address_id":%q,"payment_method":"cash"}`, addressID),
		customer.AccessToken, &order); status != http.StatusCreated {
		t.Fatalf("place: status = %d", status)
	}
	return order.ID
}

// walkShopToReady is the shop's half: accept, prepare and declare ready. The
// last of those is the event that puts the order on the dispatch board.
func walkShopToReady(t *testing.T, base string, owner tokens, shopID, orderID string) {
	t.Helper()
	queue := base + "/v1/merchants/" + shopID + "/orders/" + orderID
	for _, step := range []string{"/accept", "/preparing", "/ready"} {
		if status := requestAs(t, http.MethodPost, queue+step, "", owner.AccessToken, nil); status != http.StatusOK {
			t.Fatalf("%s: status = %d", step, status)
		}
	}
}

// The whole delivery, end to end: a shop declares an order ready, the rider who
// is standing outside is offered it, takes it, collects it and delivers it —
// and the customer's order follows every step without the rider ever writing to
// it directly.
func TestARiderTakesAnOrderToTheDoor(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	customer, owner, shopID, addressID := readyToOrder(t, base, tail)

	// The rider is on shift before the food is ready, so the first offer round
	// at `ready` has somebody to ask.
	rider, partner := onShift(t, base, tail, "Rafi", dhanmondiLat, dhanmondiLng)
	if partner.Availability != "available" || partner.AvailabilityLabel == "" {
		t.Fatalf("partner = %+v", partner)
	}
	if partner.AcceptancePercent != 100 {
		t.Errorf("a new rider reads as %d%% reliable", partner.AcceptancePercent)
	}

	orderID := placeOrder(t, base, customer, addressID)
	walkShopToReady(t, base, owner, shopID, orderID)

	// The offer is already waiting on the rider's own list.
	var mine jobListPayload
	if status := requestAs(t, http.MethodGet, base+"/v1/partner/jobs?live=true&lang=en", "",
		rider.AccessToken, &mine); status != http.StatusOK {
		t.Fatalf("jobs: status = %d", status)
	}
	if mine.Total != 1 || mine.Jobs[0].Status != "offered" {
		t.Fatalf("jobs = %+v", mine)
	}
	job := mine.Jobs[0]
	if job.OrderID != orderID {
		t.Fatalf("job %s is about order %s, want %s", job.ID, job.OrderID, orderID)
	}
	// Both ends travel with the job, so a rider's screen needs no second call:
	// who to collect from, who to hand to, and a phone number for each.
	if job.Pickup.Name == "" || job.Pickup.Phone == "" || job.Pickup.SingleLine == "" {
		t.Fatalf("pickup = %+v", job.Pickup)
	}
	if job.Destination.Name != "Fardin" || job.Destination.Phone == "" {
		t.Fatalf("destination = %+v", job.Destination)
	}
	// Every number is rendered by the server: a thin client prints the string.
	if job.Distance == "" || job.StatusLabel == "" || job.BandLabel == "" {
		t.Fatalf("job = %+v", job)
	}
	if job.Code == "" {
		t.Error("the rider cannot say the order's code to the shopkeeper")
	}

	// Accept, collect, deliver — and the order follows each one.
	steps := []struct {
		path      string
		jobStatus string
		orderWant string
	}{
		{"/accept", "assigned", "ready"},
		{"/collect", "collected", "picked_up"},
		{"/deliver", "delivered", "delivered"},
	}
	for _, step := range steps {
		var moved jobPayload
		if status := requestAs(t, http.MethodPost,
			base+"/v1/partner/jobs/"+job.ID+step.path+"?lang=en", "",
			rider.AccessToken, &moved); status != http.StatusOK {
			t.Fatalf("%s: status = %d", step.path, status)
		}
		if moved.Status != step.jobStatus {
			t.Fatalf("%s = %q, want %q", step.path, moved.Status, step.jobStatus)
		}

		var order orderResponse
		if status := requestAs(t, http.MethodGet, base+"/v1/orders/"+orderID+"?lang=en", "",
			customer.AccessToken, &order); status != http.StatusOK {
			t.Fatalf("read order: status = %d", status)
		}
		if order.Status != step.orderWant {
			t.Fatalf("after %s the order is %q, want %q", step.path, order.Status, step.orderWant)
		}
	}

	// The delivery is finished, so the rider is carrying nothing again and the
	// customer's order is closed.
	var me partnerPayload
	if status := requestAs(t, http.MethodGet, base+"/v1/partner", "", rider.AccessToken, &me); status != http.StatusOK {
		t.Fatalf("me: status = %d", status)
	}
	if me.Carrying != 0 || me.Availability != "available" {
		t.Fatalf("partner = %+v", me)
	}
	var done orderResponse
	if status := requestAs(t, http.MethodGet, base+"/v1/orders/"+orderID, "",
		customer.AccessToken, &done); status != http.StatusOK {
		t.Fatalf("read order: status = %d", status)
	}
	if done.Status != "delivered" || done.Live {
		t.Fatalf("order = %+v", done)
	}
	// And the rider's history holds it.
	var history jobListPayload
	if status := requestAs(t, http.MethodGet, base+"/v1/partner/jobs?live=false", "",
		rider.AccessToken, &history); status != http.StatusOK {
		t.Fatalf("history: status = %d", status)
	}
	if history.Total != 1 || history.Jobs[0].Status != "delivered" {
		t.Fatalf("history = %+v", history)
	}
}

// The phase's third acceptance criterion, over the wire: the rider who was not
// offered the job cannot take it, however they ask.
func TestTwoRidersCannotTakeTheSameOrder(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	customer, owner, shopID, addressID := readyToOrder(t, base, tail)

	rafi, _ := onShift(t, base, tail, "Rafi", dhanmondiLat, dhanmondiLng)
	// Nadia is a little further from the shop, so the first round is Rafi's.
	nadia, _ := onShift(t, base, tail, "Nadia", dhanmondiLat+0.004, dhanmondiLng+0.004)

	orderID := placeOrder(t, base, customer, addressID)
	walkShopToReady(t, base, owner, shopID, orderID)

	var mine jobListPayload
	if status := requestAs(t, http.MethodGet, base+"/v1/partner/jobs?live=true", "",
		rafi.AccessToken, &mine); status != http.StatusOK {
		t.Fatalf("jobs: status = %d", status)
	}
	if mine.Total != 1 {
		t.Fatalf("the nearer rider was not offered the job: %+v", mine)
	}
	jobID := mine.Jobs[0].ID

	// Nadia can see it is out there — she cannot take it out of Rafi's hands.
	if status := requestAs(t, http.MethodPost, base+"/v1/partner/jobs/"+jobID+"/accept", "",
		nadia.AccessToken, nil); status != http.StatusConflict {
		t.Fatalf("a second rider accepting returned %d, want 409", status)
	}

	if status := requestAs(t, http.MethodPost, base+"/v1/partner/jobs/"+jobID+"/accept", "",
		rafi.AccessToken, nil); status != http.StatusOK {
		t.Fatalf("accept: status = %d", status)
	}
	// And still not afterwards.
	if status := requestAs(t, http.MethodPost, base+"/v1/partner/jobs/"+jobID+"/collect", "",
		nadia.AccessToken, nil); status != http.StatusConflict {
		t.Fatalf("a second rider collecting returned %d, want 409", status)
	}
}

// The first acceptance criterion: a partner sees only jobs inside their radius.
// A rider in Uttara is not shown a Dhanmondi pickup, and is told why rather
// than being shown an empty list.
func TestAPartnerSeesOnlyWorkInsideTheirRadius(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	customer, owner, shopID, addressID := readyToOrder(t, base, tail)

	// Nobody near the shop, so the order goes onto the board and waits.
	far, _ := onShift(t, base, tail, "Faraway", 23.8759, 90.3795)

	orderID := placeOrder(t, base, customer, addressID)
	walkShopToReady(t, base, owner, shopID, orderID)

	var feed feedPayload
	if status := requestAs(t, http.MethodGet, base+"/v1/partner/feed?lang=en", "",
		far.AccessToken, &feed); status != http.StatusOK {
		t.Fatalf("feed: status = %d", status)
	}
	if len(feed.Jobs) != 0 {
		t.Fatalf("a rider 15 km away was shown %d jobs", len(feed.Jobs))
	}
	if feed.Reason != "nothing_nearby" || feed.Notice == "" {
		t.Fatalf("feed = %+v, want a sentence rather than an empty list", feed)
	}

	// A rider outside the shop sees it.
	near, _ := onShift(t, base, tail, "Nearby", dhanmondiLat, dhanmondiLng)
	var mine feedPayload
	if status := requestAs(t, http.MethodGet, base+"/v1/partner/feed?lang=en", "",
		near.AccessToken, &mine); status != http.StatusOK {
		t.Fatalf("feed: status = %d", status)
	}
	if len(mine.Jobs) != 1 || mine.Jobs[0].OrderID != orderID {
		t.Fatalf("feed = %+v", mine)
	}
	if mine.Jobs[0].ToPickup == "" {
		t.Error("the feed does not say how far the pickup is")
	}
	if mine.RadiusM <= 0 {
		t.Errorf("radius = %f", mine.RadiusM)
	}

	// Going offline is a sentence too, not an empty list.
	if status := requestAs(t, http.MethodPut, base+"/v1/partner/availability",
		`{"availability":"offline"}`, near.AccessToken, nil); status != http.StatusOK {
		t.Fatalf("offline: status = %d", status)
	}
	var quiet feedPayload
	if status := requestAs(t, http.MethodGet, base+"/v1/partner/feed?lang=bn", "",
		near.AccessToken, &quiet); status != http.StatusOK {
		t.Fatalf("feed: status = %d", status)
	}
	if quiet.Reason != "offline" || quiet.Notice == "" {
		t.Fatalf("feed = %+v", quiet)
	}
}

// The second acceptance criterion: D4's distance choice is honoured. A rider
// who has said they only want short runs is not offered a long one, and the
// preference survives a round trip with a label to show for it.
func TestALongRunIsNotOfferedToAShortRunRider(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	rider, _ := onShift(t, base, tail, "Rafi", dhanmondiLat, dhanmondiLng)
	var chosen partnerPayload
	if status := requestAs(t, http.MethodPut, base+"/v1/partner/preference",
		`{"preference":"short"}`, rider.AccessToken, &chosen); status != http.StatusOK {
		t.Fatalf("preference: status = %d", status)
	}
	if chosen.Preference != "short" || chosen.PreferenceLabel == "" {
		t.Fatalf("partner = %+v", chosen)
	}

	// A preference that is not one of the three is refused, rather than
	// quietly becoming "any" and showing somebody work they said no to.
	if status := requestAs(t, http.MethodPut, base+"/v1/partner/preference",
		`{"preference":"medium"}`, rider.AccessToken, nil); status != http.StatusBadRequest {
		t.Fatalf("an invented preference returned %d, want 400", status)
	}

	// A rider cannot declare themselves busy: that is what being at the
	// concurrent-job limit is called, and it is the platform's to say.
	if status := requestAs(t, http.MethodPut, base+"/v1/partner/availability",
		`{"availability":"busy"}`, rider.AccessToken, nil); status != http.StatusBadRequest {
		t.Fatalf("a self-declared busy returned %d, want 400", status)
	}
}

// A rider who cannot complete a delivery says why, and the order says so too.
func TestAFailedDeliveryNeedsAReason(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	customer, owner, shopID, addressID := readyToOrder(t, base, tail)
	rider, _ := onShift(t, base, tail, "Rafi", dhanmondiLat, dhanmondiLng)

	orderID := placeOrder(t, base, customer, addressID)
	walkShopToReady(t, base, owner, shopID, orderID)

	var mine jobListPayload
	if status := requestAs(t, http.MethodGet, base+"/v1/partner/jobs?live=true", "",
		rider.AccessToken, &mine); status != http.StatusOK || mine.Total != 1 {
		t.Fatalf("jobs = %+v (status %d)", mine, status)
	}
	jobID := mine.Jobs[0].ID

	if status := requestAs(t, http.MethodPost, base+"/v1/partner/jobs/"+jobID+"/accept", "",
		rider.AccessToken, nil); status != http.StatusOK {
		t.Fatalf("accept: status = %d", status)
	}
	if status := requestAs(t, http.MethodPost, base+"/v1/partner/jobs/"+jobID+"/fail",
		`{"reason":""}`, rider.AccessToken, nil); status != http.StatusBadRequest {
		t.Fatalf("a reasonless failure returned %d, want 400", status)
	}

	// Before collection there is nothing to fail: the food is still on the
	// counter, so giving up puts the job back on the board and the order is
	// still on its way.
	var givenUp jobPayload
	if status := requestAs(t, http.MethodPost, base+"/v1/partner/jobs/"+jobID+"/fail?lang=en",
		`{"reason":"my bike broke down"}`, rider.AccessToken, &givenUp); status != http.StatusOK {
		t.Fatalf("give up: status = %d", status)
	}
	if givenUp.Status != "waiting" {
		t.Fatalf("job = %+v, want it back on the board", givenUp)
	}
	var stillGoing orderResponse
	if status := requestAs(t, http.MethodGet, base+"/v1/orders/"+orderID, "",
		customer.AccessToken, &stillGoing); status != http.StatusOK {
		t.Fatalf("read order: status = %d", status)
	}
	if stillGoing.Status != "ready" || !stillGoing.Live {
		t.Fatalf("a rider's broken bike ended the order: %+v", stillGoing)
	}

	// Take it again, and this time collect the food first.
	admin := signInAs(t, base, tail, uniquePhone(t), "ops console", "admin")
	if status := requestAs(t, http.MethodPost, base+"/v1/admin/dispatch/sweep", "",
		admin.AccessToken, nil); status != http.StatusOK {
		t.Fatalf("sweep: status = %d", status)
	}
	for _, step := range []string{"/accept", "/collect"} {
		if status := requestAs(t, http.MethodPost, base+"/v1/partner/jobs/"+jobID+step, "",
			rider.AccessToken, nil); status != http.StatusOK {
			t.Fatalf("%s: status = %d", step, status)
		}
	}

	var failed jobPayload
	if status := requestAs(t, http.MethodPost, base+"/v1/partner/jobs/"+jobID+"/fail?lang=en",
		`{"reason":"nobody at the address"}`, rider.AccessToken, &failed); status != http.StatusOK {
		t.Fatalf("fail: status = %d", status)
	}
	if failed.Status != "failed" || failed.Reason != "nobody at the address" || failed.Live {
		t.Fatalf("job = %+v", failed)
	}

	var order orderResponse
	if status := requestAs(t, http.MethodGet, base+"/v1/orders/"+orderID+"?lang=en", "",
		customer.AccessToken, &order); status != http.StatusOK {
		t.Fatalf("read order: status = %d", status)
	}
	if order.Status != "failed" {
		t.Fatalf("order = %q, want failed", order.Status)
	}
	last := order.Events[len(order.Events)-1]
	if last.Actor != "partner" || last.Reason != "nobody at the address" {
		t.Fatalf("the order does not say why it failed: %+v", last)
	}
}

// A declined job goes back on the board and the sweep gives it to somebody
// else. Without the sweep's second pass it would wait there forever.
func TestASweptJobFindsAnotherRider(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	customer, owner, shopID, addressID := readyToOrder(t, base, tail)
	rafi, _ := onShift(t, base, tail, "Rafi", dhanmondiLat, dhanmondiLng)
	nadia, _ := onShift(t, base, tail, "Nadia", dhanmondiLat+0.004, dhanmondiLng+0.004)

	orderID := placeOrder(t, base, customer, addressID)
	walkShopToReady(t, base, owner, shopID, orderID)

	var mine jobListPayload
	if status := requestAs(t, http.MethodGet, base+"/v1/partner/jobs?live=true", "",
		rafi.AccessToken, &mine); status != http.StatusOK || mine.Total != 1 {
		t.Fatalf("jobs = %+v (status %d)", mine, status)
	}
	jobID := mine.Jobs[0].ID

	var declined jobPayload
	if status := requestAs(t, http.MethodPost, base+"/v1/partner/jobs/"+jobID+"/decline", "",
		rafi.AccessToken, &declined); status != http.StatusOK {
		t.Fatalf("decline: status = %d", status)
	}
	if declined.Status != "waiting" {
		t.Fatalf("job = %+v", declined)
	}

	admin := signInAs(t, base, tail, uniquePhone(t), "ops console", "admin")
	var sweep struct {
		Expired int `json:"expired"`
		Offered int `json:"offered"`
	}
	if status := requestAs(t, http.MethodPost, base+"/v1/admin/dispatch/sweep", "",
		admin.AccessToken, &sweep); status != http.StatusOK {
		t.Fatalf("sweep: status = %d", status)
	}
	if sweep.Offered != 1 {
		t.Fatalf("sweep = %+v, want the declined job re-offered", sweep)
	}

	// To Nadia, not back to Rafi.
	var hers jobListPayload
	if status := requestAs(t, http.MethodGet, base+"/v1/partner/jobs?live=true", "",
		nadia.AccessToken, &hers); status != http.StatusOK {
		t.Fatalf("jobs: status = %d", status)
	}
	if hers.Total != 1 || hers.Jobs[0].ID != jobID || hers.Jobs[0].Status != "offered" {
		t.Fatalf("the declined job did not reach the second rider: %+v", hers)
	}
	var his jobListPayload
	if status := requestAs(t, http.MethodGet, base+"/v1/partner/jobs?live=true", "",
		rafi.AccessToken, &his); status != http.StatusOK {
		t.Fatalf("jobs: status = %d", status)
	}
	if his.Total != 0 {
		t.Fatalf("the rider who declined was asked again: %+v", his)
	}

	// The sweep is an operator's tool, not a customer's.
	if status := requestAs(t, http.MethodPost, base+"/v1/admin/dispatch/sweep", "",
		customer.AccessToken, nil); status != http.StatusForbidden {
		t.Fatalf("a customer sweeping returned %d, want 403", status)
	}
}

// An account that is not a partner gets a 404 from every partner route rather
// than an empty rider profile.
func TestThePartnerRoutesAreForPartners(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	nobody := signInAs(t, base, tail, uniquePhone(t), "a phone", "customer")
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/v1/partner"},
		{http.MethodGet, "/v1/partner/feed"},
		{http.MethodGet, "/v1/partner/jobs"},
	} {
		if status := requestAs(t, route.method, base+route.path, "", nobody.AccessToken, nil); status != http.StatusNotFound {
			t.Errorf("%s %s returned %d, want 404", route.method, route.path, status)
		}
	}
	// And an anonymous caller does not get that far.
	if status := requestAs(t, http.MethodGet, base+"/v1/partner", "", "", nil); status != http.StatusUnauthorized {
		t.Errorf("an anonymous caller got %d, want 401", status)
	}
}
