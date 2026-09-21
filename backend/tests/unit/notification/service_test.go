package notification

import (
	"context"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/notification/contract"
)

// TestTheContractDelivers is what order actually depends on —
// NotificationContract, not this package's own types.
func TestTheContractDelivers(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	var api contract.NotificationContract = r.service
	r.identity.phones["usr_1"] = "+8801700000000"

	if err := api.Notify(ctx, "usr_1", "Title", "Body"); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if len(r.sms.sends) != 1 {
		t.Fatalf("sms sends = %v", r.sms.sends)
	}
}
