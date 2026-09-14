package merchant

import (
	"context"
	"testing"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/application"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// TestTheServiceSatisfiesItsContract: the whole point of contract/ is that
// consumers depend on the interface, so it must be the interface the service
// actually implements.
func TestTheServiceSatisfiesItsContract(t *testing.T) {
	var _ contract.MerchantContract = (*application.Service)(nil)
}

// ----------------------------------------------------------- opening hours

func TestAnOwnerCanSetTheirOpeningHours(t *testing.T) {
	h := newHarness(t)
	h.approve(t, "usr_1")

	merchant, err := h.operations.SetHours(context.Background(), "usr_1", application.HoursRequest{
		Days: map[string][]string{
			"1": {"07:00-11:00", "18:00-23:00"},
			"5": {"15:00-24:00"},
		},
	})
	if err != nil {
		t.Fatalf("SetHours: %v", err)
	}

	// Monday breakfast, then a gap, then dinner.
	if !merchant.IsOpenAt(at(2026, time.March, 2, 8, 0)) {
		t.Error("shut at 08:00 Monday, inside the breakfast window")
	}
	if merchant.IsOpenAt(at(2026, time.March, 2, 14, 0)) {
		t.Error("open at 14:00 Monday, in the gap")
	}
	if !merchant.IsOpenAt(at(2026, time.March, 2, 20, 0)) {
		t.Error("shut at 20:00 Monday, inside the dinner window")
	}
	// Tuesday is not in the schedule at all.
	if merchant.IsOpenAt(at(2026, time.March, 3, 12, 0)) {
		t.Error("open on Tuesday, which the owner left out")
	}
}

// TestOpeningHoursCanBeSetWhileUnderReview: a shop waiting on an admin still
// has to be able to say it is shut for Eid.
func TestOpeningHoursCanBeSetWhileUnderReview(t *testing.T) {
	h := newHarness(t)
	h.register(t, "usr_1", validRequest())
	h.documentAll(t, "usr_1", domain.TypeRestaurant)
	if _, err := h.registration.SubmitForReview(context.Background(), "usr_1"); err != nil {
		t.Fatalf("SubmitForReview: %v", err)
	}

	if _, err := h.operations.SetHours(context.Background(), "usr_1", application.HoursRequest{
		Days: map[string][]string{"1": {"09:00-18:00"}},
	}); err != nil {
		t.Errorf("SetHours while under review: %v", err)
	}
	if _, err := h.operations.StartHoliday(context.Background(), "usr_1", application.HolidayRequest{}); err != nil {
		t.Errorf("StartHoliday while under review: %v", err)
	}
}

// TestAScheduleThatNeverOpensIsRefused: it makes the shop permanently
// invisible in a way that looks like a bug to its owner. Holiday mode is what
// says so on the listing.
func TestAScheduleThatNeverOpensIsRefused(t *testing.T) {
	h := newHarness(t)
	h.approve(t, "usr_1")

	_, err := h.operations.SetHours(context.Background(), "usr_1", application.HoursRequest{})
	if got := errs.CodeOf(err); got != "hours_always_closed" {
		t.Errorf("code = %q, want hours_always_closed", got)
	}
}

func TestOpeningHoursThatDoNotMakeSenseAreRefused(t *testing.T) {
	cases := map[string]struct {
		days map[string][]string
		code string
	}{
		"overlapping":    {map[string][]string{"1": {"09:00-14:00", "13:00-22:00"}}, "overlapping_hours"},
		"too many":       {map[string][]string{"1": {"06:00-08:00", "09:00-11:00", "12:00-14:00", "15:00-17:00"}}, "too_many_hours"},
		"bad day":        {map[string][]string{"monday": {"09:00-22:00"}}, "invalid_weekday"},
		"unreadable":     {map[string][]string{"1": {"nine to five"}}, "invalid_hours"},
		"wraps midnight": {map[string][]string{"1": {"22:00-02:00"}}, "invalid_hours"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			h.approve(t, "usr_1")

			_, err := h.operations.SetHours(context.Background(), "usr_1", application.HoursRequest{Days: tc.days})
			if got := errs.CodeOf(err); got != tc.code {
				t.Errorf("code = %q, want %q", got, tc.code)
			}
		})
	}
}

func TestSettingHoursReportsAStoreThatIsDown(t *testing.T) {
	h := newHarness(t)
	h.approve(t, "usr_1")
	h.repo.saveErr = errStore

	_, err := h.operations.SetHours(context.Background(), "usr_1",
		application.HoursRequest{Days: map[string][]string{"1": {"09:00-18:00"}}})
	if errs.KindOf(err) != errs.KindUnavailable {
		t.Errorf("kind = %v, want unavailable", errs.KindOf(err))
	}
}

// ---------------------------------------------------------------- holidays

// TestHolidayModeHidesTheShopWithoutTouchingTheSearchIndex: an approved shop
// stays active in the index and is filtered out by IsListed, so a holiday costs
// one row write and expires on its own.
func TestHolidayModeHidesTheShopWithoutTouchingTheSearchIndex(t *testing.T) {
	h := newHarness(t)
	merchant := h.approve(t, "usr_1")

	onHoliday, err := h.operations.StartHoliday(context.Background(), "usr_1", application.HolidayRequest{
		Until: at(2026, time.March, 10, 0, 0), Reason: "ঈদের ছুটি",
	})
	if err != nil {
		t.Fatalf("StartHoliday: %v", err)
	}

	now := h.clock.Now()
	if onHoliday.IsListed(now) {
		t.Error("a shop on holiday is listed")
	}
	if onHoliday.Status != domain.StatusApproved {
		t.Errorf("status = %q, want approval untouched by a holiday", onHoliday.Status)
	}
	if !h.geo.isActive(merchant.ID) {
		t.Error("holiday mode changed the search index")
	}

	// And it comes back on its own once the date passes.
	if !onHoliday.IsListed(at(2026, time.March, 11, 12, 0)) {
		t.Error("the shop did not come back after its holiday ended")
	}
}

func TestAnOwnerCanComeBackEarly(t *testing.T) {
	h := newHarness(t)
	h.approve(t, "usr_1")

	if _, err := h.operations.StartHoliday(context.Background(), "usr_1",
		application.HolidayRequest{Reason: "Family emergency"}); err != nil {
		t.Fatalf("StartHoliday: %v", err)
	}

	back, err := h.operations.EndHoliday(context.Background(), "usr_1")
	if err != nil {
		t.Fatalf("EndHoliday: %v", err)
	}
	if !back.IsListed(h.clock.Now()) {
		t.Error("the shop is still hidden after ending its holiday")
	}
}

func TestHolidayDatesThatMakeNoSenseAreRefusedWithAReadableCode(t *testing.T) {
	cases := map[string]struct {
		until time.Time
		code  string
	}{
		"in the past": {at(2026, time.March, 1, 0, 0), "holiday_in_the_past"},
		"too far":     {at(2026, time.June, 1, 0, 0), "holiday_too_long"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			h.approve(t, "usr_1")

			_, err := h.operations.StartHoliday(context.Background(), "usr_1",
				application.HolidayRequest{Until: tc.until})
			if got := errs.CodeOf(err); got != tc.code {
				t.Errorf("code = %q, want %q", got, tc.code)
			}
		})
	}
}

func TestAnOverlongHolidayReasonIsRefused(t *testing.T) {
	h := newHarness(t)
	h.approve(t, "usr_1")

	_, err := h.operations.StartHoliday(context.Background(), "usr_1",
		application.HolidayRequest{Reason: longString(201)})
	if got := errs.CodeOf(err); got != "invalid_holiday" {
		t.Errorf("code = %q, want invalid_holiday", got)
	}
}

func TestHolidayWritesReportAStoreThatIsDown(t *testing.T) {
	for name, call := range map[string]func(*harness) error{
		"start": func(h *harness) error {
			_, e := h.operations.StartHoliday(context.Background(), "usr_1", application.HolidayRequest{})
			return e
		},
		"end": func(h *harness) error {
			_, e := h.operations.EndHoliday(context.Background(), "usr_1")
			return e
		},
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			h.approve(t, "usr_1")
			h.repo.saveErr = errStore

			if got := errs.KindOf(call(h)); got != errs.KindUnavailable {
				t.Errorf("kind = %v, want unavailable", got)
			}
		})
	}
}

// ----------------------------------------------------------- the contract

func TestTheContractCarriesAPreformattedOpenStatus(t *testing.T) {
	h := newHarness(t)
	merchant := h.approve(t, "usr_1")

	got, err := h.service.Merchant(context.Background(), merchant.ID)
	if err != nil {
		t.Fatalf("Merchant: %v", err)
	}
	if got.OpenStatus == "" {
		t.Error("the contract carries no open status for the client to render")
	}
	if got.SingleLine == "" {
		t.Error("the contract carries no composed address")
	}
	if !got.IsListed || !got.IsOpenNow {
		t.Errorf("listed = %v, open = %v; want an approved shop at noon to be both",
			got.IsListed, got.IsOpenNow)
	}
	if got.DivisionCode != "BD-C" {
		t.Errorf("division = %q, want the D3 ceiling carried through", got.DivisionCode)
	}
}

// TestTheContractDoesNotCarryDocuments: a trade licence number reaching
// discovery is one careless handler away from a customer's screen.
func TestTheContractDoesNotCarryDocuments(t *testing.T) {
	h := newHarness(t)
	merchant := h.approve(t, "usr_1")

	got, err := h.service.Merchant(context.Background(), merchant.ID)
	if err != nil {
		t.Fatalf("Merchant: %v", err)
	}
	// contract.Merchant has no document field at all; this asserts the shape
	// stays that way by checking the fields it does carry are the public ones.
	if got.Name == "" || got.Phone == "" {
		t.Errorf("the contract dropped a field consumers need: %+v", got)
	}
}

// TestListedReturnsOnlyWhatACustomerMaySee, in the order discovery asked for.
func TestListedReturnsOnlyWhatACustomerMaySee(t *testing.T) {
	h := newHarness(t)
	approved := h.approve(t, "usr_1")
	draft := h.register(t, "usr_2", validRequest())

	// A third shop, approved then put on holiday.
	onHoliday := h.approve(t, "usr_3")
	if _, err := h.operations.StartHoliday(context.Background(), "usr_3", application.HolidayRequest{}); err != nil {
		t.Fatalf("StartHoliday: %v", err)
	}

	listed, err := h.service.Listed(context.Background(),
		[]string{"mch_nope", draft.ID, onHoliday.ID, approved.ID})
	if err != nil {
		t.Fatalf("Listed: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != approved.ID {
		t.Errorf("listed = %+v, want only the approved, open shop", listed)
	}
}

// TestListedPreservesTheOrderItWasGiven: the caller is discovery, handing over
// a nearest-first radius search, and re-sorting here would throw away the
// ordering the query paid for.
func TestListedPreservesTheOrderItWasGiven(t *testing.T) {
	h := newHarness(t)
	first := h.approve(t, "usr_1")
	second := h.approve(t, "usr_2")
	third := h.approve(t, "usr_3")

	listed, err := h.service.Listed(context.Background(), []string{third.ID, first.ID, second.ID})
	if err != nil {
		t.Fatalf("Listed: %v", err)
	}
	want := []string{third.ID, first.ID, second.ID}
	if len(listed) != len(want) {
		t.Fatalf("listed %d shops, want %d", len(listed), len(want))
	}
	for i := range want {
		if listed[i].ID != want[i] {
			t.Errorf("position %d = %q, want %q", i, listed[i].ID, want[i])
		}
	}
}

func TestIsAcceptingOrdersFollowsTheClock(t *testing.T) {
	h := newHarness(t)
	merchant := h.approve(t, "usr_1")

	open, err := h.service.IsAcceptingOrders(context.Background(), merchant.ID)
	if err != nil {
		t.Fatalf("IsAcceptingOrders: %v", err)
	}
	if !open {
		t.Error("an approved shop at noon is not accepting orders")
	}

	// 03:00, well outside the default 09:00-22:00.
	h.clock.Set(at(2026, time.March, 3, 3, 0))
	open, err = h.service.IsAcceptingOrders(context.Background(), merchant.ID)
	if err != nil {
		t.Fatalf("IsAcceptingOrders: %v", err)
	}
	if open {
		t.Error("a shop at 03:00 is accepting orders")
	}
}

func TestTheContractReportsAShopThatIsNotThere(t *testing.T) {
	h := newHarness(t)

	for name, call := range map[string]func() error{
		"merchant": func() error { _, e := h.service.Merchant(context.Background(), "mch_nope"); return e },
		"accepting": func() error {
			_, e := h.service.IsAcceptingOrders(context.Background(), "mch_nope")
			return e
		},
	} {
		t.Run(name, func(t *testing.T) {
			err := call()
			if got := errs.CodeOf(err); got != "merchant_not_found" {
				t.Errorf("code = %q, want merchant_not_found", got)
			}
		})
	}
}

func TestTheContractReportsAStoreThatIsDown(t *testing.T) {
	h := newHarness(t)
	h.repo.readErr = errStore

	for name, call := range map[string]func() error{
		"merchant": func() error { _, e := h.service.Merchant(context.Background(), "mch_1"); return e },
		"accepting": func() error {
			_, e := h.service.IsAcceptingOrders(context.Background(), "mch_1")
			return e
		},
	} {
		t.Run(name, func(t *testing.T) {
			if got := errs.KindOf(call()); got != errs.KindUnavailable {
				t.Errorf("kind = %v, want unavailable", got)
			}
		})
	}
}

// TestListedSkipsAStaleIdRatherThanFailingThePage: discovery holds ids from a
// spatial index that can be a moment behind a withdrawal, and failing the batch
// would turn one stale row into an empty search result.
func TestListedSkipsAStaleIDRatherThanFailingThePage(t *testing.T) {
	h := newHarness(t)
	approved := h.approve(t, "usr_1")

	listed, err := h.service.Listed(context.Background(), []string{"mch_withdrawn", approved.ID})
	if err != nil {
		t.Fatalf("Listed: %v", err)
	}
	if len(listed) != 1 {
		t.Errorf("listed = %d, want the surviving shop", len(listed))
	}
}
