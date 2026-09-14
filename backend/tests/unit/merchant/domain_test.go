// Package merchant tests the merchant domain and use cases.
package merchant

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/domain"
)

// dhaka is the timezone every opening hour in this product is expressed in.
var dhaka = time.FixedZone("+06", 6*60*60)

// at builds an instant in Bangladesh local time, so a test that says "Monday at
// nine" means what a shop owner would mean by it.
func at(year int, month time.Month, day, hour, minute int) time.Time {
	return time.Date(year, month, day, hour, minute, 0, 0, dhaka)
}

// validDetails is a complete, valid set of shop details.
func validDetails() domain.Details {
	pin, err := domain.NewPin(23.7509, 90.3925)
	if err != nil {
		panic(err)
	}
	return domain.Details{
		Name:  "নূরজাহান হোটেল",
		Type:  domain.TypeRestaurant,
		Phone: "01712345678",
		Line1: "12/A, Mirpur Road",
		Pin:   pin,
	}
}

// ------------------------------------------------------------------ types

func TestTypesRoundTripThroughTheirCodes(t *testing.T) {
	for _, kind := range domain.AllTypes() {
		parsed, err := domain.ParseType(kind.String())
		if err != nil {
			t.Fatalf("ParseType(%q): %v", kind, err)
		}
		if parsed != kind {
			t.Errorf("ParseType(%q) = %q", kind, parsed)
		}
	}
}

func TestTypesAreReadCaseInsensitivelyAndTrimmed(t *testing.T) {
	parsed, err := domain.ParseType("  PHARMACY ")
	if err != nil {
		t.Fatalf("ParseType: %v", err)
	}
	if parsed != domain.TypePharmacy {
		t.Errorf("type = %q", parsed)
	}
}

func TestAnUnknownTypeIsRefused(t *testing.T) {
	if _, err := domain.ParseType("hardware"); !errors.Is(err, domain.ErrUnknownType) {
		t.Errorf("error = %v, want ErrUnknownType", err)
	}
}

func TestStatusesRoundTripThroughTheirCodes(t *testing.T) {
	for _, status := range domain.AllStatuses() {
		parsed, err := domain.ParseStatus(status.String())
		if err != nil {
			t.Fatalf("ParseStatus(%q): %v", status, err)
		}
		if parsed != status {
			t.Errorf("ParseStatus(%q) = %q", status, parsed)
		}
	}
	if _, err := domain.ParseStatus("banished"); !errors.Is(err, domain.ErrUnknownStatus) {
		t.Errorf("error = %v, want ErrUnknownStatus", err)
	}
}

// ------------------------------------------------------- the workflow table

// TestTheApprovalWorkflowAllowsExactlyTheseMoves pins the whole transition
// table. The set of legal moves is a product decision, and this test is what
// makes changing it deliberate rather than incidental.
func TestTheApprovalWorkflowAllowsExactlyTheseMoves(t *testing.T) {
	allowed := map[domain.Status][]domain.Status{
		domain.StatusDraft:         {domain.StatusPendingReview},
		domain.StatusPendingReview: {domain.StatusApproved, domain.StatusRejected},
		domain.StatusRejected:      {domain.StatusPendingReview},
		domain.StatusApproved:      {domain.StatusSuspended},
		domain.StatusSuspended:     {domain.StatusApproved},
	}

	for _, from := range domain.AllStatuses() {
		permitted := map[domain.Status]bool{}
		for _, to := range allowed[from] {
			permitted[to] = true
		}
		for _, to := range domain.AllStatuses() {
			if got := from.CanTransitionTo(to); got != permitted[to] {
				t.Errorf("%s -> %s: allowed = %v, want %v", from, to, got, permitted[to])
			}
		}
		if got := from.NextStatuses(); len(got) != len(allowed[from]) {
			t.Errorf("%s.NextStatuses() = %v, want %v", from, got, allowed[from])
		}
	}
}

// TestNoStatusMayTransitionToItself: a re-approval that looks like a no-op
// would still write an audit event claiming a decision was taken.
func TestNoStatusMayTransitionToItself(t *testing.T) {
	for _, status := range domain.AllStatuses() {
		if status.CanTransitionTo(status) {
			t.Errorf("%s may transition to itself", status)
		}
	}
}

// TestNextStatusesCannotBeEditedByItsCaller: the table is shared, and a caller
// that appended to the returned slice would corrupt it for everyone.
func TestNextStatusesCannotBeEditedByItsCaller(t *testing.T) {
	next := domain.StatusPendingReview.NextStatuses()
	if len(next) == 0 {
		t.Fatal("pending_review has no next statuses")
	}
	next[0] = domain.StatusDraft

	if again := domain.StatusPendingReview.NextStatuses(); again[0] == domain.StatusDraft {
		t.Error("editing the returned slice changed the transition table")
	}
}

// ------------------------------------------------------------- registration

// TestAShopRegistersFromAnywhereInBangladesh is the phase's first acceptance
// criterion (D1). The same details register a shop in the capital and in a
// remote upazila; nothing in the domain looks at where anyone else is.
func TestAShopRegistersFromAnywhereInBangladesh(t *testing.T) {
	places := map[string][2]float64{
		"Dhaka, Dhanmondi":    {23.7465, 90.3760},
		"Rangpur, Pirganj":    {25.5153, 89.3306},
		"Bandarban, Thanchi":  {21.7500, 92.4167},
		"Khulna, Koyra":       {22.3450, 89.2900},
		"Sylhet, Companiganj": {25.0800, 91.6300},
	}

	for place, coords := range places {
		pin, err := domain.NewPin(coords[0], coords[1])
		if err != nil {
			t.Fatalf("%s: NewPin: %v", place, err)
		}
		details := validDetails()
		details.Pin = pin

		merchant, err := domain.NewMerchant("mch_1", "usr_1", details, at(2026, time.March, 2, 10, 0))
		if err != nil {
			t.Errorf("%s: NewMerchant: %v", place, err)
			continue
		}
		if merchant.Status != domain.StatusDraft {
			t.Errorf("%s: status = %q, want draft", place, merchant.Status)
		}
	}
}

func TestAShopNeedsAnOwner(t *testing.T) {
	if _, err := domain.NewMerchant("mch_1", "   ", validDetails(), time.Now()); !errors.Is(err, domain.ErrEmptyOwner) {
		t.Errorf("error = %v, want ErrEmptyOwner", err)
	}
}

func TestANewShopIsNotVisibleToAnyone(t *testing.T) {
	now := at(2026, time.March, 2, 12, 0)
	merchant, err := domain.NewMerchant("mch_1", "usr_1", validDetails(), now)
	if err != nil {
		t.Fatalf("NewMerchant: %v", err)
	}
	if merchant.IsListed(now) {
		t.Error("a shop in draft is listed")
	}
	if merchant.IsOpenAt(now) {
		t.Error("a shop in draft is taking orders")
	}
}

// TestANewShopGetsWorkingOpeningHours: a shop registered with no hours would be
// approved and still invisible, with nothing to tell its owner why.
func TestANewShopGetsWorkingOpeningHours(t *testing.T) {
	merchant, err := domain.NewMerchant("mch_1", "usr_1", validDetails(), time.Now())
	if err != nil {
		t.Fatalf("NewMerchant: %v", err)
	}
	if merchant.Hours.IsAlwaysClosed() {
		t.Error("a new shop has no opening hours at all")
	}
}

func TestShopDetailsAreValidated(t *testing.T) {
	longName := strings.Repeat("অ", 121)
	longLine := strings.Repeat("a", 201)

	cases := map[string]struct {
		mutate func(*domain.Details)
		want   error
	}{
		"no name":             {func(d *domain.Details) { d.Name = "  " }, domain.ErrEmptyName},
		"name too long":       {func(d *domain.Details) { d.Name = longName }, domain.ErrNameTooLong},
		"unknown type":        {func(d *domain.Details) { d.Type = "hardware" }, domain.ErrUnknownType},
		"no phone":            {func(d *domain.Details) { d.Phone = "" }, domain.ErrInvalidPhone},
		"landline":            {func(d *domain.Details) { d.Phone = "0212345678" }, domain.ErrInvalidPhone},
		"bad email":           {func(d *domain.Details) { d.Email = "not-an-email" }, domain.ErrInvalidEmail},
		"insecure logo":       {func(d *domain.Details) { d.LogoURL = "http://example.com/l.png" }, domain.ErrInvalidLogoURL},
		"no address":          {func(d *domain.Details) { d.Line1 = "   " }, domain.ErrEmptyAddressLine},
		"address line 1 long": {func(d *domain.Details) { d.Line1 = longLine }, domain.ErrTooLong},
		"address line 2 long": {func(d *domain.Details) { d.Line2 = longLine }, domain.ErrTooLong},
		"no pin":              {func(d *domain.Details) { d.Pin = domain.Pin{} }, domain.ErrInvalidPin},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			details := validDetails()
			tc.mutate(&details)
			if _, err := domain.NewMerchant("mch_1", "usr_1", details, time.Now()); !errors.Is(err, tc.want) {
				t.Errorf("error = %v, want %v", err, tc.want)
			}
		})
	}
}

// TestAnOverlongEmailIsRefusedEvenIfItLooksWellFormed: the 254-byte limit is
// what an SMTP envelope accepts, and a longer address is one receipts will
// bounce off.
func TestAnOverlongEmailIsRefusedEvenIfItLooksWellFormed(t *testing.T) {
	details := validDetails()
	details.Email = strings.Repeat("a", 250) + "@example.com"
	if _, err := domain.NewMerchant("mch_1", "usr_1", details, time.Now()); !errors.Is(err, domain.ErrInvalidEmail) {
		t.Errorf("error = %v, want ErrInvalidEmail", err)
	}
}

func TestShopDetailsAreTrimmedAndNormalised(t *testing.T) {
	details := validDetails()
	details.Name = "  Nurjahan Hotel  "
	details.Email = "  Shop@Example.COM "
	details.Line1 = "  12/A, Mirpur Road "
	details.Line2 = "  Level 2 "

	merchant, err := domain.NewMerchant("mch_1", "usr_1", details, time.Now())
	if err != nil {
		t.Fatalf("NewMerchant: %v", err)
	}
	if merchant.Name != "Nurjahan Hotel" {
		t.Errorf("name = %q", merchant.Name)
	}
	if merchant.Email != "shop@example.com" {
		t.Errorf("email = %q", merchant.Email)
	}
	if merchant.Line1 != "12/A, Mirpur Road" || merchant.Line2 != "Level 2" {
		t.Errorf("address = %q / %q", merchant.Line1, merchant.Line2)
	}
}

func TestAnEmptyEmailIsAllowed(t *testing.T) {
	details := validDetails()
	details.Email = "   "
	merchant, err := domain.NewMerchant("mch_1", "usr_1", details, time.Now())
	if err != nil {
		t.Fatalf("NewMerchant: %v", err)
	}
	if merchant.Email != "" {
		t.Errorf("email = %q, want empty", merchant.Email)
	}
}

func TestLogoAddressesWeAccept(t *testing.T) {
	cases := map[string]struct {
		in      string
		want    string
		refused bool
	}{
		"empty":             {"", "", false},
		"served path":       {"/static/demo/logo.png", "/static/demo/logo.png", false},
		"https":             {"https://cdn.example.com/l.png", "https://cdn.example.com/l.png", false},
		"plain http":        {"http://cdn.example.com/l.png", "", true},
		"bare https":        {"https://", "", true},
		"relative":          {"logo.png", "", true},
		"javascript":        {"javascript:alert(1)", "", true},
		"protocol-relative": {"//cdn.example.com/l.png", "", true},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			details := validDetails()
			details.LogoURL = tc.in
			merchant, err := domain.NewMerchant("mch_1", "usr_1", details, time.Now())
			if tc.refused {
				if !errors.Is(err, domain.ErrInvalidLogoURL) {
					t.Errorf("error = %v, want ErrInvalidLogoURL", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewMerchant: %v", err)
			}
			if merchant.LogoURL != tc.want {
				t.Errorf("logo = %q, want %q", merchant.LogoURL, tc.want)
			}
		})
	}
}

// --------------------------------------------------------------- phone

func TestOneNumberHasOneSpelling(t *testing.T) {
	spellings := []string{
		"01712345678",
		"+8801712345678",
		"8801712345678",
		"01712-345678",
		"017 1234 5678",
		"1712345678",
	}
	for _, spelling := range spellings {
		normalised, err := domain.NormalisePhone(spelling)
		if err != nil {
			t.Errorf("NormalisePhone(%q): %v", spelling, err)
			continue
		}
		if normalised != "+8801712345678" {
			t.Errorf("NormalisePhone(%q) = %q", spelling, normalised)
		}
	}
}

func TestNumbersThatAreNotBangladeshiMobilesAreRefused(t *testing.T) {
	for _, raw := range []string{"", "0171234567", "017123456789", "0212345678", "01212345678", "+14155550123"} {
		if _, err := domain.NormalisePhone(raw); !errors.Is(err, domain.ErrInvalidPhone) {
			t.Errorf("NormalisePhone(%q) error = %v, want ErrInvalidPhone", raw, err)
		}
	}
}

// ----------------------------------------------------------------- pins

func TestAPinMustBeAPlace(t *testing.T) {
	cases := map[string][2]float64{
		"latitude past the pole": {91, 90},
		"latitude below":         {-91, 90},
		"longitude past":         {23, 181},
		"longitude below":        {23, -181},
		"null island":            {0, 0},
	}
	for name, coords := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := domain.NewPin(coords[0], coords[1]); !errors.Is(err, domain.ErrInvalidPin) {
				t.Errorf("error = %v, want ErrInvalidPin", err)
			}
		})
	}
}

func TestAPlacementKnowsWhetherItIsPlaced(t *testing.T) {
	if (domain.Placement{}).IsPlaced() {
		t.Error("an empty placement reports itself placed")
	}
	if !(domain.Placement{DivisionCode: "BD-C"}).IsPlaced() {
		t.Error("a placement with a division reports itself unplaced")
	}
}

// ------------------------------------------------------------ status moves

func TestAStatusChangeRefusesAnIllegalMove(t *testing.T) {
	merchant := approvedMerchant(t)
	if _, err := merchant.WithStatus(domain.StatusPendingReview, ""); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Errorf("error = %v, want ErrInvalidTransition", err)
	}
}

// TestAnApprovalClearsAPreviousRejectionReason: a live shop shown beside the
// text explaining why it was refused is worse than no explanation at all.
func TestAnApprovalClearsAPreviousRejectionReason(t *testing.T) {
	merchant := submittedMerchant(t)

	rejected, err := merchant.WithStatus(domain.StatusRejected, "The trade licence photo is unreadable.")
	if err != nil {
		t.Fatalf("reject: %v", err)
	}
	resubmitted, err := rejected.WithStatus(domain.StatusPendingReview, "")
	if err != nil {
		t.Fatalf("resubmit: %v", err)
	}
	approved, err := resubmitted.WithStatus(domain.StatusApproved, "")
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if approved.ReviewNote != "" {
		t.Errorf("review note = %q, want empty after approval", approved.ReviewNote)
	}
}

func TestAReviewNoteHasALimit(t *testing.T) {
	merchant := submittedMerchant(t)
	if _, err := merchant.WithStatus(domain.StatusRejected, strings.Repeat("অ", 501)); !errors.Is(err, domain.ErrTooLong) {
		t.Errorf("error = %v, want ErrTooLong", err)
	}
}

// -------------------------------------------------------------- visibility

// TestApprovalGatesVisibility is the phase's second acceptance criterion. A
// shop is visible only once an admin has approved it, and stops being visible
// the moment one suspends it.
func TestApprovalGatesVisibility(t *testing.T) {
	now := at(2026, time.March, 2, 12, 0) // Monday noon, inside default hours
	merchant := submittedMerchant(t)

	if merchant.IsListed(now) {
		t.Fatal("a shop awaiting review is listed")
	}

	approved, err := merchant.WithStatus(domain.StatusApproved, "")
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if !approved.IsListed(now) {
		t.Error("an approved shop is not listed")
	}
	if !approved.IsOpenAt(now) {
		t.Error("an approved shop inside its opening hours is not taking orders")
	}

	suspended, err := approved.WithStatus(domain.StatusSuspended, "Repeated cancellations.")
	if err != nil {
		t.Fatalf("suspend: %v", err)
	}
	if suspended.IsListed(now) {
		t.Error("a suspended shop is still listed")
	}
}

// TestAClosedShopIsStillListed: a customer scrolling at midnight should find
// the restaurant and see when it opens, rather than have it vanish.
func TestAClosedShopIsStillListed(t *testing.T) {
	merchant := approvedMerchant(t)
	midnight := at(2026, time.March, 2, 2, 0)

	if !merchant.IsListed(midnight) {
		t.Error("an approved shop outside its hours is not listed")
	}
	if merchant.IsOpenAt(midnight) {
		t.Error("a shop outside its hours is taking orders")
	}
}

func TestASingleLineAddressIsComposedByTheServer(t *testing.T) {
	merchant := approvedMerchant(t)
	merchant.Line2 = "Level 2"
	merchant = merchant.WithPlacement(domain.Placement{AreaName: "Dhanmondi", DivisionCode: "BD-C"})

	if got, want := merchant.SingleLine(), "12/A, Mirpur Road, Level 2, Dhanmondi"; got != want {
		t.Errorf("SingleLine() = %q, want %q", got, want)
	}

	merchant.Line2 = ""
	merchant = merchant.WithPlacement(domain.Placement{DivisionCode: "BD-C"})
	if got, want := merchant.SingleLine(), "12/A, Mirpur Road"; got != want {
		t.Errorf("SingleLine() = %q, want %q", got, want)
	}
}

// ------------------------------------------------------------- open status

// TestTheServerComposesTheOpenStatusLine: every client shows the same words,
// because a client that decided for itself would decide differently (2.9).
func TestTheServerComposesTheOpenStatusLine(t *testing.T) {
	approved := approvedMerchant(t)
	draft := submittedMerchant(t)

	holiday, err := domain.NewHoliday(at(2026, time.March, 10, 0, 0), "Eid",
		at(2026, time.March, 2, 12, 0))
	if err != nil {
		t.Fatalf("NewHoliday: %v", err)
	}
	onHoliday := approved.WithHoliday(holiday)

	cases := []struct {
		name     string
		merchant domain.Merchant
		now      time.Time
		en, bn   string
	}{
		{"not approved", draft, at(2026, time.March, 2, 12, 0), "Not available", "এখন বন্ধ"},
		{"on holiday", onHoliday, at(2026, time.March, 2, 12, 0), "On holiday", "ছুটিতে আছে"},
		{"open", approved, at(2026, time.March, 2, 12, 0), "Open until 22:00", "খোলা আছে — বন্ধ হবে 22:00"},
		{"closed for now", approved, at(2026, time.March, 2, 2, 0), "Opens at 09:00", "বন্ধ — খুলবে 09:00"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.merchant.OpenStatus(tc.now, "en"); got != tc.en {
				t.Errorf("OpenStatus(en) = %q, want %q", got, tc.en)
			}
			if got := tc.merchant.OpenStatus(tc.now, "bn"); got != tc.bn {
				t.Errorf("OpenStatus(bn) = %q, want %q", got, tc.bn)
			}
		})
	}
}

// TestTheOpenStatusIsBengaliByDefault: the audience is Bengali-first (1.4), so
// an unrecognised language code gets Bengali rather than English.
func TestTheOpenStatusIsBengaliByDefault(t *testing.T) {
	merchant := approvedMerchant(t)
	now := at(2026, time.March, 2, 12, 0)

	if got := merchant.OpenStatus(now, ""); got != merchant.OpenStatus(now, "bn") {
		t.Errorf("OpenStatus with no language = %q, want the Bengali line", got)
	}
}

// TestAShopWithNoClosingTimeSaysOnlyThatItIsOpen covers a 24-hour shop: there
// is no useful "until", so the line says nothing it cannot back up.
func TestAShopOpenAroundTheClockSaysOnlyThatItIsOpen(t *testing.T) {
	merchant := approvedMerchant(t)

	// A window covering the whole day, with no other day open, so NextOpening
	// finds nothing later either.
	allDay, err := domain.NewWindow(0, 24*60)
	if err != nil {
		t.Fatalf("NewWindow: %v", err)
	}
	hours, err := domain.NewWeeklyHours(map[time.Weekday][]domain.Window{time.Monday: {allDay}})
	if err != nil {
		t.Fatalf("NewWeeklyHours: %v", err)
	}
	merchant = merchant.WithHours(hours)

	// Monday, inside the window. ClosingAfter returns 24:00, which is a real
	// closing time, so the line names it.
	now := at(2026, time.March, 2, 12, 0)
	if got, want := merchant.OpenStatus(now, "en"), "Open until 24:00"; got != want {
		t.Errorf("OpenStatus = %q, want %q", got, want)
	}

	// Tuesday: nothing is open, and nothing opens in the next week either.
	tuesday := at(2026, time.March, 3, 12, 0)
	if got, want := merchant.OpenStatus(tuesday, "en"), "Opens at 00:00"; got != want {
		t.Errorf("OpenStatus = %q, want %q", got, want)
	}
}

// helpers ------------------------------------------------------------------

// approvedMerchant is a fully documented, approved shop with default hours.
func approvedMerchant(t *testing.T) domain.Merchant {
	t.Helper()
	merchant, err := submittedMerchant(t).WithStatus(domain.StatusApproved, "")
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	return merchant
}

// submittedMerchant is a placed, fully documented shop awaiting review.
func submittedMerchant(t *testing.T) domain.Merchant {
	t.Helper()
	merchant, err := domain.NewMerchant("mch_1", "usr_1", validDetails(), at(2026, time.March, 1, 9, 0))
	if err != nil {
		t.Fatalf("NewMerchant: %v", err)
	}
	merchant = merchant.WithPlacement(domain.Placement{
		AreaCode: "BD-C-DHA-01", AreaName: "Dhanmondi",
		DistrictCode: "BD-C-DHA", DivisionCode: "BD-C",
	})
	for _, kind := range domain.RequiredDocuments(merchant.Type) {
		document, err := domain.NewDocument(kind, "NUM-"+kind.String(), "/static/demo/doc.png",
			at(2026, time.March, 1, 10, 0))
		if err != nil {
			t.Fatalf("NewDocument(%s): %v", kind, err)
		}
		merchant = merchant.WithDocument(document)
	}
	submitted, err := merchant.WithStatus(domain.StatusPendingReview, "")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	return submitted
}

// TestAShopWithNoScheduleAtAllSaysOnlyThatItIsClosed: a schedule that never
// opens has no "opens at" to offer, and the line must not invent one.
func TestAShopWithNoScheduleAtAllSaysOnlyThatItIsClosed(t *testing.T) {
	hours, err := domain.NewWeeklyHours(nil)
	if err != nil {
		t.Fatalf("NewWeeklyHours: %v", err)
	}
	merchant := approvedMerchant(t).WithHours(hours)
	now := at(2026, time.March, 2, 12, 0)

	if got, want := merchant.OpenStatus(now, "en"), "Closed"; got != want {
		t.Errorf("OpenStatus(en) = %q, want %q", got, want)
	}
	if got, want := merchant.OpenStatus(now, "bn"), "বন্ধ"; got != want {
		t.Errorf("OpenStatus(bn) = %q, want %q", got, want)
	}
}

// TestEmailShapesWeAcceptAndRefuse. The check is deliberately loose — the only
// way to know an address works is to send to it, and a strict pattern turns a
// real customer away over an apostrophe — but it catches what is plainly
// unusable.
func TestEmailShapesWeAcceptAndRefuse(t *testing.T) {
	accepted := []string{
		"shop@example.com",
		"shop+orders@example.co.uk",
		"o'brien@example.com",
	}
	refused := []string{
		"no-at-sign.com",
		"@example.com",
		"shop@",
		"shop@@example.com",
		"shop@example",
		"shop@.example.com",
		"shop@example.",
		"shop name@example.com",
		"shop<script>@example.com",
		"shop\t@example.com",
	}

	for _, email := range accepted {
		details := validDetails()
		details.Email = email
		if _, err := domain.NewMerchant("mch_1", "usr_1", details, time.Now()); err != nil {
			t.Errorf("%q was refused: %v", email, err)
		}
	}
	for _, email := range refused {
		details := validDetails()
		details.Email = email
		if _, err := domain.NewMerchant("mch_1", "usr_1", details, time.Now()); !errors.Is(err, domain.ErrInvalidEmail) {
			t.Errorf("%q: error = %v, want ErrInvalidEmail", email, err)
		}
	}
}
