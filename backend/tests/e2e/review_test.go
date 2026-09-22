package e2e

import (
	"fmt"
	"net/http"
	"testing"
)

// deliveredOrderWithParties walks a real order from cart to delivered
// through the same steps the shop, the rider and the customer actually take
// — the same helpers dispatch's and tracking's own E2E suites use — and
// returns everyone review needs to be tested against: the customer, the
// shop, the item and the rider who delivered it.
func deliveredOrderWithParties(t *testing.T) (base string, tail *logTail, stop func(), customer tokens, shopID, itemID, orderID, partnerID string) {
	t.Helper()
	base, tail, stopAPI := startAuthAPI(t)

	shopID, owner, itemID := openAllHours(t, base, tail)
	customer = signInAs(t, base, tail, uniquePhone(t), "customer phone", "customer")

	var address struct {
		ID string `json:"id"`
	}
	if status := requestAs(t, http.MethodPost, base+"/v1/me/addresses", fmt.Sprintf(`{
		"label": "Home", "recipient_name": "Fardin", "recipient_phone": "01711111111",
		"line1": "House 5, Road 3", "lat": %f, "lng": %f
	}`, dhanmondiLat, dhanmondiLng), customer.AccessToken, &address); status != http.StatusCreated {
		t.Fatalf("create address: status = %d", status)
	}
	if status := requestAs(t, http.MethodPost, base+"/v1/cart/items",
		fmt.Sprintf(`{"merchant_id":%q,"kind":"item","target_id":%q,"quantity":1}`, shopID, itemID),
		customer.AccessToken, nil); status != http.StatusOK {
		t.Fatalf("fill cart: status = %d", status)
	}
	if status := requestAs(t, http.MethodPut, base+"/v1/cart/address",
		fmt.Sprintf(`{"address_id":%q,"lat":%f,"lng":%f}`, address.ID, dhanmondiLat, dhanmondiLng),
		customer.AccessToken, nil); status != http.StatusOK {
		t.Fatalf("place cart: status = %d", status)
	}

	// The rider is on shift before the food is ready, so the first offer
	// round at `ready` has somebody to ask — the same ordering dispatch's
	// own E2E suite uses.
	rider, partner := onShift(t, base, tail, "Karim", dhanmondiLat, dhanmondiLng)
	partnerID = partner.ID

	orderID = placeOrder(t, base, customer, address.ID)
	walkShopToReady(t, base, owner, shopID, orderID)

	var mine jobListPayload
	if status := requestAs(t, http.MethodGet, base+"/v1/partner/jobs?live=true", "",
		rider.AccessToken, &mine); status != http.StatusOK || mine.Total != 1 {
		t.Fatalf("jobs: status = %d, mine = %+v", status, mine)
	}
	job := mine.Jobs[0]
	for _, step := range []string{"/accept", "/collect", "/deliver"} {
		if status := requestAs(t, http.MethodPost, base+"/v1/partner/jobs/"+job.ID+step, "",
			rider.AccessToken, nil); status != http.StatusOK {
			t.Fatalf("%s: status = %d", step, status)
		}
	}

	return base, tail, stopAPI, customer, shopID, itemID, orderID, partnerID
}

type reviewResponse struct {
	ID        string `json:"id"`
	Subject   string `json:"subject"`
	SubjectID string `json:"subject_id"`
	Rating    int    `json:"rating"`
}

type ratingResponse struct {
	Average float64 `json:"average"`
	Count   int     `json:"count"`
}

// TestACustomerRatesEveryPartyInADeliveredOrder is review's whole point,
// end to end against the real binary and real Postgres: a customer whose
// order actually finished can rate the shop, the item and the rider who
// carried it, and each rating is readable back through the public,
// unauthenticated-looking surface as soon as it lands.
func TestACustomerRatesEveryPartyInADeliveredOrder(t *testing.T) {
	base, _, stop, customer, shopID, itemID, orderID, partnerID := deliveredOrderWithParties(t)
	defer stop()

	cases := []struct {
		subject   string
		subjectID string
	}{
		{"merchant", shopID},
		{"item", itemID},
		{"partner", partnerID},
	}
	for _, c := range cases {
		var review reviewResponse
		status := requestAs(t, http.MethodPost, base+"/v1/reviews", fmt.Sprintf(`{
			"order_id": %q, "subject": %q, "subject_id": %q, "rating": 5, "comment": "excellent"
		}`, orderID, c.subject, c.subjectID), customer.AccessToken, &review)
		if status != http.StatusOK {
			t.Fatalf("submit %s review: status = %d", c.subject, status)
		}
		if review.Subject != c.subject || review.SubjectID != c.subjectID || review.Rating != 5 {
			t.Fatalf("%s review = %+v", c.subject, review)
		}

		var rating ratingResponse
		status = requestAs(t, http.MethodGet,
			base+"/v1/ratings?subject="+c.subject+"&subject_id="+c.subjectID, "",
			customer.AccessToken, &rating)
		if status != http.StatusOK {
			t.Fatalf("read %s rating: status = %d", c.subject, status)
		}
		if rating.Average != 5 || rating.Count != 1 {
			t.Fatalf("%s rating = %+v, want average 5 across 1 review", c.subject, rating)
		}
	}
}

// TestAReviewCannotBeSubmittedTwiceOrByAStranger proves the two eligibility
// rules that matter most end to end: it is the order's own customer's
// opinion, and only once.
func TestAReviewCannotBeSubmittedTwiceOrByAStranger(t *testing.T) {
	base, tail, stop, customer, shopID, _, orderID, _ := deliveredOrderWithParties(t)
	defer stop()

	body := fmt.Sprintf(`{"order_id":%q,"subject":"merchant","subject_id":%q,"rating":4}`, orderID, shopID)
	if status := requestAs(t, http.MethodPost, base+"/v1/reviews", body, customer.AccessToken, nil); status != http.StatusOK {
		t.Fatalf("first review: status = %d", status)
	}
	if status := requestAs(t, http.MethodPost, base+"/v1/reviews", body, customer.AccessToken, nil); status != http.StatusConflict {
		t.Fatalf("duplicate review: status = %d, want 409", status)
	}

	stranger := signInAs(t, base, tail, uniquePhone(t), "stranger's phone", "customer")
	if status := requestAs(t, http.MethodPost, base+"/v1/reviews", body, stranger.AccessToken, nil); status != http.StatusForbidden {
		t.Fatalf("a stranger's review: status = %d, want 403", status)
	}
}

// TestSupportTicketLifecycleOverHTTP walks a ticket from a customer raising
// it through an admin resolving it — the "support" half of the phase,
// proven against the real binary rather than a fake resolve use case.
func TestSupportTicketLifecycleOverHTTP(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	customer, owner, shopID, addressID := readyToOrder(t, base, tail)
	orderID := placeOrder(t, base, customer, addressID)
	walkShopToReady(t, base, owner, shopID, orderID)

	var ticket struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	status := requestAs(t, http.MethodPost, base+"/v1/support/tickets",
		fmt.Sprintf(`{"order_id":%q,"subject":"never arrived"}`, orderID),
		customer.AccessToken, &ticket)
	if status != http.StatusOK || ticket.Status != "open" {
		t.Fatalf("raise ticket: status = %d, ticket = %+v", status, ticket)
	}

	var mine struct {
		Tickets []struct {
			ID string `json:"id"`
		} `json:"tickets"`
	}
	if status := requestAs(t, http.MethodGet, base+"/v1/me/support/tickets", "",
		customer.AccessToken, &mine); status != http.StatusOK || len(mine.Tickets) != 1 {
		t.Fatalf("my tickets: status = %d, mine = %+v", status, mine)
	}

	admin := signInAs(t, base, tail, uniquePhone(t), "admin console", "admin")
	var queue struct {
		Tickets []struct {
			ID string `json:"id"`
		} `json:"tickets"`
	}
	if status := requestAs(t, http.MethodGet, base+"/v1/admin/support/tickets", "",
		admin.AccessToken, &queue); status != http.StatusOK {
		t.Fatalf("open queue: status = %d", status)
	}
	found := false
	for _, tk := range queue.Tickets {
		found = found || tk.ID == ticket.ID
	}
	if !found {
		t.Fatalf("ticket %s not in the open queue: %+v", ticket.ID, queue)
	}

	var resolved struct {
		Status     string `json:"status"`
		Resolution string `json:"resolution"`
	}
	status = requestAs(t, http.MethodPost, base+"/v1/admin/support/tickets/resolve",
		fmt.Sprintf(`{"ticket_id":%q,"resolution":"rejected","note":"delivered on time per rider log"}`, ticket.ID),
		admin.AccessToken, &resolved)
	if status != http.StatusOK || resolved.Status != "resolved" || resolved.Resolution != "rejected" {
		t.Fatalf("resolve: status = %d, resolved = %+v", status, resolved)
	}

	// A resolved ticket leaves the queue an agent works down.
	queue.Tickets = nil
	if status := requestAs(t, http.MethodGet, base+"/v1/admin/support/tickets", "",
		admin.AccessToken, &queue); status != http.StatusOK {
		t.Fatalf("open queue after resolving: status = %d", status)
	}
	for _, tk := range queue.Tickets {
		if tk.ID == ticket.ID {
			t.Fatalf("a resolved ticket is still in the open queue: %+v", queue)
		}
	}
}

// TestSupportTicketsAreAdminOnly: the queue and the resolve action are not
// something a customer, even the one who raised the ticket, may reach.
func TestSupportTicketsAreAdminOnly(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	customer, owner, shopID, addressID := readyToOrder(t, base, tail)
	orderID := placeOrder(t, base, customer, addressID)
	walkShopToReady(t, base, owner, shopID, orderID)

	var ticket struct {
		ID string `json:"id"`
	}
	if status := requestAs(t, http.MethodPost, base+"/v1/support/tickets",
		fmt.Sprintf(`{"order_id":%q,"subject":"never arrived"}`, orderID),
		customer.AccessToken, &ticket); status != http.StatusOK {
		t.Fatalf("raise ticket: status = %d", status)
	}

	if status := requestAs(t, http.MethodGet, base+"/v1/admin/support/tickets", "",
		customer.AccessToken, nil); status != http.StatusForbidden {
		t.Fatalf("customer reading the queue: status = %d, want 403", status)
	}
	if status := requestAs(t, http.MethodPost, base+"/v1/admin/support/tickets/resolve",
		fmt.Sprintf(`{"ticket_id":%q,"resolution":"rejected"}`, ticket.ID),
		customer.AccessToken, nil); status != http.StatusForbidden {
		t.Fatalf("customer resolving their own ticket: status = %d, want 403", status)
	}
	_ = tail
}
