package order

import (
	"context"
	"net/http"
	"testing"
	"time"

	cfgcontract "github.com/rootlogic-lab/delivery/backend/internal/modules/config/contract"
	discocontract "github.com/rootlogic-lab/delivery/backend/internal/modules/discovery/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/order/application"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/order/domain"
	usercontract "github.com/rootlogic-lab/delivery/backend/internal/modules/user/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// The corners. Each of these is a sentence somebody reads, a fallback somebody
// depends on, or a refusal that has to name itself correctly.

// An address with no separate recipient falls back to the customer's own name.
// A delivery slip reading "" is a rider knocking on a door with nothing to say.
func TestAnAddressWithNoRecipientUsesTheCustomersName(t *testing.T) {
	r := newRig()
	r.user.address = usercontract.Address{
		ID: "ADR-1", Label: "Home", Line1: "House 5",
		SingleLine: "House 5, Dhanmondi", Lat: 23.746, Lng: 90.375,
		AreaCode: "DHA-DHK-DHN",
	}
	r.user.profile = usercontract.Profile{UserID: "USR-1", DisplayName: "অতিথি"}

	view := place(t, r, "")
	if view.Destination.Name != "অতিথি" {
		t.Fatalf("recipient = %q, want the customer's display name", view.Destination.Name)
	}
}

// Both reasons an address can be undeliverable, in both languages. The
// permanent one and the "too far for now" one read differently on purpose.
func TestTheUndeliverableSentences(t *testing.T) {
	cases := []struct {
		reason, lang, want string
	}{
		{discocontract.ReasonOutsideDivision, "en",
			"That address is in another division, so this shop cannot deliver to it."},
		{discocontract.ReasonOutsideDivision, "",
			"এই ঠিকানা অন্য বিভাগে। এই দোকান থেকে সেখানে ডেলিভারি করা যাবে না।"},
		{discocontract.ReasonBeyondMaxRadius, "en", "That address is too far from this shop."},
		{discocontract.ReasonBeyondMaxRadius, "", "এই ঠিকানা দোকান থেকে অনেক দূরে।"},
	}
	for _, tc := range cases {
		r := newRig()
		r.discovery.reach = discocontract.Reach{Reason: tc.reason}
		_, err := r.place.Execute(context.Background(), "USR-1", application.PlaceRequest{
			AddressID: "ADR-1", Payment: "cash", Lang: tc.lang,
		})
		if errs.CodeOf(err) != "not_deliverable" {
			t.Fatalf("%s/%s: err = %v", tc.reason, tc.lang, err)
		}
		if got := errs.MessageOf(err); got != tc.want {
			t.Errorf("%s/%s: message = %q, want %q", tc.reason, tc.lang, got, tc.want)
		}
	}
}

// Every reason a cancellation is refused has a sentence in both languages.
func TestTheCancellationSentences(t *testing.T) {
	cases := []struct {
		name             string
		prepare          func(*testing.T, *rig, string)
		reason           string
		bengali, english string
	}{
		{"the window has closed", func(_ *testing.T, r *rig, _ string) {
			r.clock.at = placedAt.Add(time.Hour)
		}, domain.ReasonTooLate,
			"বাতিল করার সময় পেরিয়ে গেছে।", "The time to cancel has passed."},

		{"the kitchen has started", func(t *testing.T, r *rig, id string) {
			move(t, r, id, domain.StatusAccepted, "", shopkeeper())
			move(t, r, id, domain.StatusPreparing, "", shopkeeper())
		}, domain.ReasonUnderway,
			"দোকান অর্ডার তৈরি শুরু করেছে, তাই এখন বাতিল করা যাবে না।",
			"The shop has started preparing this, so it can no longer be cancelled."},

		{"it has already finished", func(t *testing.T, r *rig, id string) {
			move(t, r, id, domain.StatusCancelled, "", customer())
		}, domain.ReasonAlreadyFinished,
			"এই অর্ডার শেষ হয়ে গেছে।", "That order has already finished."},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for lang, want := range map[string]string{"": tc.bengali, "en": tc.english} {
				r := newRig()
				placed := place(t, r, "")
				tc.prepare(t, r, placed.ID)

				got, err := r.transitions.CancelStatus(context.Background(), placed.ID,
					application.Caller{Actor: domain.ActorCustomer, ID: "USR-1", Lang: lang})
				if err != nil {
					t.Fatalf("CancelStatus: %v", err)
				}
				if got.Allowed || got.Reason != tc.reason {
					t.Fatalf("cancel = %+v, want %s", got, tc.reason)
				}
				if got.Text != want {
					t.Errorf("lang %q: text = %q, want %q", lang, got.Text, want)
				}
			}
		})
	}
}

// An order that has finished cannot be moved again, whoever asks.
func TestMovingAFinishedOrder(t *testing.T) {
	r := newRig()
	placed := place(t, r, "")
	move(t, r, placed.ID, domain.StatusCancelled, "", customer())

	_, err := r.transitions.Execute(context.Background(), placed.ID,
		domain.StatusAccepted, "", shopkeeper())
	if errs.CodeOf(err) != "order_finished" {
		t.Fatalf("err = %v, want order_finished", err)
	}
}

// An address the user module returns empty cannot become a destination. The
// refusal names itself rather than falling through to a generic failure.
func TestAnEmptyAddressIsRefused(t *testing.T) {
	r := newRig()
	r.user.address = usercontract.Address{}
	_, err := r.place.Execute(context.Background(), "USR-1", application.PlaceRequest{
		AddressID: "ADR-1", Payment: "cash",
	})
	if errs.CodeOf(err) != "no_address" {
		t.Fatalf("err = %v, want no_address", err)
	}
}

// A draft that cannot become an order for any other reason gets one honest
// message. Reached here by a caller with no identity, which the transport
// refuses but the use case must not assume.
func TestADraftWithNoCustomer(t *testing.T) {
	r := newRig()
	_, err := r.place.Execute(context.Background(), "", application.PlaceRequest{
		AddressID: "ADR-1", Payment: "cash",
	})
	if errs.CodeOf(err) != "invalid_order" {
		t.Fatalf("err = %v, want invalid_order", err)
	}
}

// A missing setting is an outage rather than a default. An order priced against
// a hard-coded ceiling would be a misconfigured division nobody noticed.
func TestMissingSettingsAreReported(t *testing.T) {
	ctx := context.Background()

	t.Run("the COD limit", func(t *testing.T) {
		r := newRig()
		settings := appendixB()
		settings.failOn = cfgcontract.OrderCODLimit
		r.config.settings = settings
		_, err := r.place.Execute(ctx, "USR-1", application.PlaceRequest{
			AddressID: "ADR-1", Payment: "cash",
		})
		if errs.CodeOf(err) != "order_config_unavailable" {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("the cancellation window, on a read", func(t *testing.T) {
		r := newRig()
		placed := place(t, r, "")
		settings := appendixB()
		settings.failOn = cfgcontract.OrderCancellationWindow
		r.config.settings = settings

		// The order still comes back; only the countdown is missing. A screen
		// with no countdown is usable; a screen with no order is not.
		view, err := r.reads.One(ctx, placed.ID, customer())
		if err != nil {
			t.Fatalf("One: %v", err)
		}
		if view.ID != placed.ID || view.Cancel.Allowed {
			t.Fatalf("view = %+v", view)
		}
	})
}

// CancelStatus is subject to the same "is this yours" check as everything else.
func TestCancelStatusIsScopedToTheOwner(t *testing.T) {
	r := newRig()
	placed := place(t, r, "")

	stranger := application.Caller{Actor: domain.ActorCustomer, ID: "USR-2"}
	if _, err := r.transitions.CancelStatus(context.Background(), placed.ID, stranger); !errs.Is(err, errs.KindNotFound) {
		t.Fatalf("err = %v, want a not-found", err)
	}
}

// Both receipt languages, on a receipt that carries every row it can.
func TestTheReceiptSpeaksBothLanguages(t *testing.T) {
	r := newRig()
	r.discovery.reach = discocontract.Reach{
		Reachable: true, AreaCode: "DHA-DHK-DHN", DistrictCode: "DHA-DHK", DivisionCode: "DHA",
		DistanceM: 8000, RequiredLevel: 1, Expanded: true,
	}
	expanded := place(t, r, "")

	want := map[string]map[string]string{
		"": {
			"subtotal": "পণ্যের মোট", "delivery": "ডেলিভারি চার্জ",
			"expansion_surcharge": "দূরত্বের জন্য অতিরিক্ত", "total": "সর্বমোট",
		},
		"en": {
			"subtotal": "Items", "delivery": "Delivery",
			"expansion_surcharge": "Extra distance charge", "total": "Total",
		},
	}
	for lang, labels := range want {
		view, err := r.reads.One(context.Background(), expanded.ID,
			application.Caller{Actor: domain.ActorCustomer, ID: "USR-1", Lang: lang})
		if err != nil {
			t.Fatalf("One: %v", err)
		}
		for _, row := range view.Receipt {
			if labels[row.Key] != "" && row.Label != labels[row.Key] {
				t.Errorf("lang %q, row %q: label = %q, want %q", lang, row.Key, row.Label, labels[row.Key])
			}
		}
	}

	// And the free-delivery row, which only a qualifying order carries.
	free := newRig()
	qualifying := place(t, free, "")
	for lang, wantLabel := range map[string]string{"": "ফ্রি ডেলিভারি", "en": "Free delivery"} {
		view, err := free.reads.One(context.Background(), qualifying.ID,
			application.Caller{Actor: domain.ActorCustomer, ID: "USR-1", Lang: lang})
		if err != nil {
			t.Fatalf("One: %v", err)
		}
		found := false
		for _, row := range view.Receipt {
			if row.Key == "free_delivery" {
				found = true
				if row.Label != wantLabel {
					t.Errorf("lang %q: label = %q, want %q", lang, row.Label, wantLabel)
				}
			}
		}
		if !found {
			t.Errorf("lang %q: receipt = %+v", lang, view.Receipt)
		}
	}
}

// A placement that fails reaches the client as a status, not a panic.
func TestAFailedPlacementOverHTTP(t *testing.T) {
	r := newRig()
	r.cart.found = false
	mux := server(r, "USR-1", nil)
	if status := call(t, mux, http.MethodPost, "/v1/orders",
		`{"address_id":"ADR-1","payment_method":"cash"}`, nil); status != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", status)
	}
}
