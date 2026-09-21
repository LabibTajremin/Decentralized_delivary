package tracking

import (
	"context"
	"testing"

	orderx "github.com/rootlogic-lab/delivery/backend/internal/modules/tracking/external/order"
)

// ascii reports whether a rendered label is plain English text. Bengali is
// the default everywhere (1.4), so a label with no non-ASCII rune in it is
// either English or a translation somebody forgot.
func ascii(s string) bool {
	for _, r := range s {
		if r > 127 {
			return false
		}
	}
	return true
}

// Every order status tracking knows how to show has a label in both
// languages.
func TestEveryStatusLabelIsRenderedInBothLanguages(t *testing.T) {
	statuses := []string{
		"pending_payment", "placed", "accepted", "preparing", "ready",
		"picked_up", "delivered", "cancelled", "rejected", "failed", "something_unknown",
	}
	ctx := context.Background()
	for _, status := range statuses {
		for _, lang := range []string{"bn", "en"} {
			r := newRig()
			r.order.orders["ord_1"] = orderx.Order{ID: "ord_1", CustomerID: "usr_1", Status: status}

			snap, err := r.snapshot.For(ctx, "ord_1", "usr_1", lang)
			if err != nil {
				t.Fatalf("status %q lang %q: For: %v", status, lang, err)
			}
			if snap.StatusLabel == "" {
				t.Fatalf("status %q lang %q: no label", status, lang)
			}
			if ascii(snap.StatusLabel) != (lang == "en") {
				t.Fatalf("status %q in %s produced %q", status, lang, snap.StatusLabel)
			}
		}
	}
}
