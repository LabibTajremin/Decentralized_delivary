package merchant

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/application"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/clock"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// harness is a wired merchant module over in-memory dependencies.
type harness struct {
	repo         *memoryRepo
	geo          *fakeGeo
	clock        *clock.Fixed
	registration *application.RegistrationUseCase
	operations   *application.OperationsUseCase
	moderation   *application.ModerationUseCase
	service      *application.Service
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	repo := newRepo()
	geo := newGeo()
	fixed := clock.NewFixed(at(2026, time.March, 2, 12, 0))
	ids := &countingIDs{}

	return &harness{
		repo:         repo,
		geo:          geo,
		clock:        fixed,
		registration: application.NewRegistrationUseCase(repo, geo, fixed, ids),
		operations:   application.NewOperationsUseCase(repo, fixed),
		moderation:   application.NewModerationUseCase(repo, geo, fixed, ids),
		service:      application.NewService(repo, fixed),
	}
}

// validRequest is a complete registration request.
func validRequest() application.DetailsRequest {
	return application.DetailsRequest{
		Name:  "নূরজাহান হোটেল",
		Type:  "restaurant",
		Phone: "01712345678",
		Line1: "12/A, Mirpur Road",
		Lat:   23.7509,
		Lng:   90.3925,
	}
}

// register creates a shop and fails the test if it could not.
func (h *harness) register(t *testing.T, ownerUserID string, req application.DetailsRequest) domain.Merchant {
	t.Helper()
	merchant, err := h.registration.Register(context.Background(), ownerUserID, req)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	return merchant
}

// documentAll attaches every document the shop's type requires.
func (h *harness) documentAll(t *testing.T, ownerUserID string, kind domain.Type) domain.Merchant {
	t.Helper()
	var merchant domain.Merchant
	for _, doc := range domain.RequiredDocuments(kind) {
		updated, err := h.registration.AddDocument(context.Background(), ownerUserID,
			application.DocumentRequest{Kind: doc.String(), Number: "N-1", FileURL: "/f.png"})
		if err != nil {
			t.Fatalf("AddDocument(%s): %v", doc, err)
		}
		merchant = updated
	}
	return merchant
}

// approve takes a shop all the way through the workflow.
func (h *harness) approve(t *testing.T, ownerUserID string) domain.Merchant {
	t.Helper()
	h.register(t, ownerUserID, validRequest())
	merchant := h.documentAll(t, ownerUserID, domain.TypeRestaurant)
	if _, err := h.registration.SubmitForReview(context.Background(), ownerUserID); err != nil {
		t.Fatalf("SubmitForReview: %v", err)
	}
	approved, err := h.moderation.Approve(context.Background(), "usr_admin", merchant.ID, "")
	if err != nil {
		t.Fatalf("Approve: %v", err)
	}
	return approved
}

// ------------------------------------------------------------- registration

// TestAMerchantCanRegisterFromAnywhereInBangladesh is the phase's acceptance
// criterion, at the use-case level: the same request registers a shop in Dhaka
// and in Thanchi, and nothing consults how many shops are already nearby.
func TestAMerchantCanRegisterFromAnywhereInBangladesh(t *testing.T) {
	places := map[string][2]float64{
		"Dhaka, Dhanmondi":   {23.7465, 90.3760},
		"Rangpur, Pirganj":   {25.5153, 89.3306},
		"Bandarban, Thanchi": {21.7500, 92.4167},
	}

	i := 0
	for place, coords := range places {
		i++
		h := newHarness(t)
		req := validRequest()
		req.Lat, req.Lng = coords[0], coords[1]

		merchant, err := h.registration.Register(context.Background(), "usr_"+place, req)
		if err != nil {
			t.Errorf("%s: Register: %v", place, err)
			continue
		}
		if merchant.Status != domain.StatusDraft {
			t.Errorf("%s: status = %q, want draft", place, merchant.Status)
		}
		if !merchant.Placement.IsPlaced() {
			t.Errorf("%s: the shop was not placed", place)
		}
	}
}

// TestANewShopIsPublishedToGeoButNotSearchable: the point goes into the index
// straight away so approving it later is a flag change, but the flag is off
// until an admin says otherwise.
func TestANewShopIsPublishedToGeoButNotSearchable(t *testing.T) {
	h := newHarness(t)
	merchant := h.register(t, "usr_1", validRequest())

	placed, ok := h.geo.placed[merchant.ID]
	if !ok {
		t.Fatal("the shop was not published to geo")
	}
	if placed.Active {
		t.Error("a shop in draft is searchable")
	}
	if placed.Lat != merchant.Pin.Lat || placed.Lng != merchant.Pin.Lng {
		t.Errorf("published at %v,%v; shop is at %v,%v",
			placed.Lat, placed.Lng, merchant.Pin.Lat, merchant.Pin.Lng)
	}
}

func TestOneAccountGetsOneShop(t *testing.T) {
	h := newHarness(t)
	h.register(t, "usr_1", validRequest())

	_, err := h.registration.Register(context.Background(), "usr_1", validRequest())
	if got := errs.CodeOf(err); got != "already_registered" {
		t.Errorf("code = %q, want already_registered", got)
	}
	if errs.KindOf(err) != errs.KindConflict {
		t.Errorf("kind = %v, want conflict", errs.KindOf(err))
	}
}

// TestTwoRegistrationsRacingLoseTheSameWay: the repository's unique constraint
// decides, and the loser gets the same answer as if it had simply arrived
// second.
func TestTwoRegistrationsRacingLoseTheSameWay(t *testing.T) {
	h := newHarness(t)
	// ByOwner finds nothing — the first write has not landed — but Create hits
	// the constraint, exactly as it would under a real race.
	h.repo.createErr = domain.ErrAlreadyRegistered

	_, err := h.registration.Register(context.Background(), "usr_1", validRequest())
	if got := errs.CodeOf(err); got != "already_registered" {
		t.Errorf("code = %q, want already_registered", got)
	}
}

func TestRegistrationRefusesAPinOutsideTheServiceArea(t *testing.T) {
	h := newHarness(t)
	h.geo.resolveErr = errs.New(errs.KindNotFound, "outside_service_area", "We do not deliver here yet.")

	_, err := h.registration.Register(context.Background(), "usr_1", validRequest())
	if got := errs.CodeOf(err); got != "outside_service_area" {
		t.Errorf("code = %q, want outside_service_area", got)
	}
	if len(h.repo.merchants) != 0 {
		t.Error("a shop was stored despite being unplaceable")
	}
}

// TestAShopIsStoredBeforeItIsPublished: the other order would leave a point in
// the search index with no shop behind it — a result discovery would return and
// nothing could resolve.
func TestAShopIsStoredBeforeItIsPublished(t *testing.T) {
	h := newHarness(t)
	h.geo.placeErr = errs.New(errs.KindUnavailable, "merchant_place_failed", "Try again.")

	_, err := h.registration.Register(context.Background(), "usr_1", validRequest())
	if err == nil {
		t.Fatal("Register succeeded despite a failed publish")
	}
	if len(h.repo.merchants) != 1 {
		t.Errorf("merchants stored = %d, want the record to survive the publish failure", len(h.repo.merchants))
	}
}

func TestRegistrationValidatesWhatTheOwnerTyped(t *testing.T) {
	cases := map[string]struct {
		mutate func(*application.DetailsRequest)
		code   string
	}{
		"no name":        {func(r *application.DetailsRequest) { r.Name = " " }, "shop_name_required"},
		"unknown type":   {func(r *application.DetailsRequest) { r.Type = "hardware" }, "unknown_merchant_type"},
		"bad phone":      {func(r *application.DetailsRequest) { r.Phone = "12345" }, "invalid_phone"},
		"bad email":      {func(r *application.DetailsRequest) { r.Email = "nope" }, "invalid_email"},
		"bad logo":       {func(r *application.DetailsRequest) { r.LogoURL = "http://x/l.png" }, "invalid_logo_url"},
		"no address":     {func(r *application.DetailsRequest) { r.Line1 = " " }, "address_line_required"},
		"unset pin":      {func(r *application.DetailsRequest) { r.Lat, r.Lng = 0, 0 }, "invalid_pin"},
		"impossible pin": {func(r *application.DetailsRequest) { r.Lat = 99 }, "invalid_pin"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			req := validRequest()
			tc.mutate(&req)

			_, err := h.registration.Register(context.Background(), "usr_1", req)
			if got := errs.CodeOf(err); got != tc.code {
				t.Errorf("code = %q, want %q", got, tc.code)
			}
			if errs.KindOf(err) != errs.KindInvalid {
				t.Errorf("kind = %v, want invalid", errs.KindOf(err))
			}
		})
	}
}

func TestTooLongDetailsAreRefusedWithOneMessage(t *testing.T) {
	h := newHarness(t)
	req := validRequest()
	req.Name = string(make([]rune, 0))
	for i := 0; i < 121; i++ {
		req.Name += "অ"
	}

	_, err := h.registration.Register(context.Background(), "usr_1", req)
	if got := errs.CodeOf(err); got != "shop_details_too_long" {
		t.Errorf("code = %q, want shop_details_too_long", got)
	}
}

func TestRegistrationReportsAStoreThatIsDown(t *testing.T) {
	for name, broken := range map[string]func(*memoryRepo){
		"owner lookup": func(r *memoryRepo) { r.byOwnerErr = errStore },
		"create":       func(r *memoryRepo) { r.createErr = errStore },
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			broken(h.repo)

			_, err := h.registration.Register(context.Background(), "usr_1", validRequest())
			if errs.KindOf(err) != errs.KindUnavailable {
				t.Errorf("kind = %v, want unavailable", errs.KindOf(err))
			}
			if !errors.Is(err, errStore) {
				t.Errorf("the cause was lost: %v", err)
			}
		})
	}
}

// ------------------------------------------------------------------ editing

func TestEditingAShopRePlacesIt(t *testing.T) {
	h := newHarness(t)
	merchant := h.register(t, "usr_1", validRequest())

	h.geo.area.AreaCode = "BD-C-DHA-02"
	h.geo.area.AreaName = "Mohammadpur"

	req := validRequest()
	req.Lat, req.Lng = 23.7600, 90.3600
	updated, err := h.registration.Update(context.Background(), "usr_1", req)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Placement.AreaCode != "BD-C-DHA-02" {
		t.Errorf("area = %q, want the re-resolved one", updated.Placement.AreaCode)
	}
	if got := h.geo.placed[merchant.ID]; got.Lat != 23.7600 {
		t.Errorf("geo still holds %v, want the moved pin", got.Lat)
	}
}

// TestAShopUnderReviewIsFrozen: an admin should decide on the details they were
// shown, not on a set that changed underneath them.
func TestAShopUnderReviewIsFrozen(t *testing.T) {
	h := newHarness(t)
	h.register(t, "usr_1", validRequest())
	h.documentAll(t, "usr_1", domain.TypeRestaurant)
	if _, err := h.registration.SubmitForReview(context.Background(), "usr_1"); err != nil {
		t.Fatalf("SubmitForReview: %v", err)
	}

	if _, err := h.registration.Update(context.Background(), "usr_1", validRequest()); errs.CodeOf(err) != "under_review" {
		t.Errorf("Update: code = %q, want under_review", errs.CodeOf(err))
	}
	_, err := h.registration.AddDocument(context.Background(), "usr_1",
		application.DocumentRequest{Kind: "trade_licence", Number: "N-2", FileURL: "/f.png"})
	if errs.CodeOf(err) != "under_review" {
		t.Errorf("AddDocument: code = %q, want under_review", errs.CodeOf(err))
	}
}

func TestAnOwnerWithNoShopIsToldSo(t *testing.T) {
	h := newHarness(t)

	for name, call := range map[string]func() error{
		"mine": func() error { _, err := h.registration.Mine(context.Background(), "usr_1"); return err },
		"update": func() error {
			_, err := h.registration.Update(context.Background(), "usr_1", validRequest())
			return err
		},
		"add doc": func() error {
			_, err := h.registration.AddDocument(context.Background(), "usr_1", application.DocumentRequest{})
			return err
		},
		"submit":   func() error { _, err := h.registration.SubmitForReview(context.Background(), "usr_1"); return err },
		"withdraw": func() error { return h.registration.Withdraw(context.Background(), "usr_1") },
		"set hours": func() error {
			_, err := h.operations.SetHours(context.Background(), "usr_1", application.HoursRequest{})
			return err
		},
		"holiday on": func() error {
			_, err := h.operations.StartHoliday(context.Background(), "usr_1", application.HolidayRequest{})
			return err
		},
		"holiday off": func() error { _, err := h.operations.EndHoliday(context.Background(), "usr_1"); return err },
	} {
		t.Run(name, func(t *testing.T) {
			err := call()
			if got := errs.CodeOf(err); got != "no_shop" {
				t.Errorf("code = %q, want no_shop", got)
			}
			if errs.KindOf(err) != errs.KindNotFound {
				t.Errorf("kind = %v, want not_found", errs.KindOf(err))
			}
		})
	}
}

func TestOwnerLookupsReportAStoreThatIsDown(t *testing.T) {
	h := newHarness(t)
	h.repo.byOwnerErr = errStore

	for name, call := range map[string]func() error{
		"registration": func() error { _, err := h.registration.Mine(context.Background(), "usr_1"); return err },
		"operations":   func() error { _, err := h.operations.EndHoliday(context.Background(), "usr_1"); return err },
	} {
		t.Run(name, func(t *testing.T) {
			if errs.KindOf(call()) != errs.KindUnavailable {
				t.Errorf("kind = %v, want unavailable", errs.KindOf(call()))
			}
		})
	}
}

func TestEditingReportsAStoreThatIsDown(t *testing.T) {
	h := newHarness(t)
	h.register(t, "usr_1", validRequest())
	h.repo.saveErr = errStore

	if _, err := h.registration.Update(context.Background(), "usr_1", validRequest()); errs.KindOf(err) != errs.KindUnavailable {
		t.Errorf("kind = %v, want unavailable", errs.KindOf(err))
	}
}

func TestEditingRefusesDetailsThatAreNotValid(t *testing.T) {
	h := newHarness(t)
	h.register(t, "usr_1", validRequest())

	req := validRequest()
	req.Lat, req.Lng = 0, 0
	if _, err := h.registration.Update(context.Background(), "usr_1", req); errs.CodeOf(err) != "invalid_pin" {
		t.Errorf("code = %q, want invalid_pin", errs.CodeOf(err))
	}

	req = validRequest()
	req.Name = " "
	if _, err := h.registration.Update(context.Background(), "usr_1", req); errs.CodeOf(err) != "shop_name_required" {
		t.Errorf("code = %q, want shop_name_required", errs.CodeOf(err))
	}
}

func TestEditingRefusesAPinThatCannotBePlaced(t *testing.T) {
	h := newHarness(t)
	h.register(t, "usr_1", validRequest())
	h.geo.resolveErr = errs.New(errs.KindNotFound, "outside_service_area", "No.")

	if _, err := h.registration.Update(context.Background(), "usr_1", validRequest()); errs.CodeOf(err) != "outside_service_area" {
		t.Errorf("code = %q, want outside_service_area", errs.CodeOf(err))
	}
}

func TestEditingReportsAFailedPublish(t *testing.T) {
	h := newHarness(t)
	h.register(t, "usr_1", validRequest())
	h.geo.placeErr = errs.New(errs.KindUnavailable, "merchant_place_failed", "Try again.")

	if _, err := h.registration.Update(context.Background(), "usr_1", validRequest()); errs.CodeOf(err) != "merchant_place_failed" {
		t.Errorf("code = %q, want merchant_place_failed", errs.CodeOf(err))
	}
}

// ---------------------------------------------------------------- documents

// TestADocumentThatThisShopDoesNotNeedIsRefused: accepting it silently would
// leave the owner waiting for an approval that was never blocked on it.
func TestADocumentThatThisShopDoesNotNeedIsRefused(t *testing.T) {
	h := newHarness(t)
	req := validRequest()
	req.Type = "grocery"
	h.register(t, "usr_1", req)

	_, err := h.registration.AddDocument(context.Background(), "usr_1",
		application.DocumentRequest{Kind: "drug_licence", Number: "D-1", FileURL: "/f.png"})
	if got := errs.CodeOf(err); got != "document_not_required" {
		t.Errorf("code = %q, want document_not_required", got)
	}
}

func TestADocumentIsValidatedBeforeItIsStored(t *testing.T) {
	h := newHarness(t)
	h.register(t, "usr_1", validRequest())

	cases := map[string]struct {
		req  application.DocumentRequest
		code string
	}{
		"unknown kind": {application.DocumentRequest{Kind: "passport", Number: "1", FileURL: "/f.png"}, "unknown_document_kind"},
		"no number":    {application.DocumentRequest{Kind: "trade_licence", Number: " ", FileURL: "/f.png"}, "document_number_required"},
		"no scan":      {application.DocumentRequest{Kind: "trade_licence", Number: "1", FileURL: " "}, "document_file_required"},
		"number long":  {application.DocumentRequest{Kind: "trade_licence", Number: longString(61), FileURL: "/f.png"}, "invalid_document"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := h.registration.AddDocument(context.Background(), "usr_1", tc.req); errs.CodeOf(err) != tc.code {
				t.Errorf("code = %q, want %q", errs.CodeOf(err), tc.code)
			}
		})
	}
}

func TestStoringADocumentReportsAStoreThatIsDown(t *testing.T) {
	h := newHarness(t)
	h.register(t, "usr_1", validRequest())
	h.repo.saveErr = errStore

	_, err := h.registration.AddDocument(context.Background(), "usr_1",
		application.DocumentRequest{Kind: "trade_licence", Number: "T-1", FileURL: "/f.png"})
	if errs.KindOf(err) != errs.KindUnavailable {
		t.Errorf("kind = %v, want unavailable", errs.KindOf(err))
	}
}

// -------------------------------------------------------------- submission

func TestSubmittingRefusesUntilEveryDocumentIsThere(t *testing.T) {
	h := newHarness(t)
	h.register(t, "usr_1", validRequest())

	_, err := h.registration.SubmitForReview(context.Background(), "usr_1")
	if got := errs.CodeOf(err); got != "documents_incomplete" {
		t.Fatalf("code = %q, want documents_incomplete", got)
	}

	// The refusal names what is missing, so the owner is told what to do.
	var structured *errs.Error
	if !errors.As(err, &structured) {
		t.Fatalf("error is not structured: %v", err)
	}
	fields := structured.Fields()
	for _, kind := range domain.RequiredDocuments(domain.TypeRestaurant) {
		if fields["missing_"+kind.String()] != "required" {
			t.Errorf("the refusal does not name %s as missing: %v", kind, fields)
		}
	}
}

func TestSubmittingRefusesAShopThatCouldNotBePlaced(t *testing.T) {
	h := newHarness(t)
	merchant := h.register(t, "usr_1", validRequest())
	h.documentAll(t, "usr_1", domain.TypeRestaurant)

	// Strip the placement behind the use case's back, the way a partially
	// migrated row would arrive.
	stored := h.repo.merchants[merchant.ID]
	stored.Placement = domain.Placement{}
	h.repo.merchants[merchant.ID] = stored

	_, err := h.registration.SubmitForReview(context.Background(), "usr_1")
	if got := errs.CodeOf(err); got != "shop_not_placed" {
		t.Errorf("code = %q, want shop_not_placed", got)
	}
}

func TestSubmittingTwiceIsRefused(t *testing.T) {
	h := newHarness(t)
	h.register(t, "usr_1", validRequest())
	h.documentAll(t, "usr_1", domain.TypeRestaurant)

	if _, err := h.registration.SubmitForReview(context.Background(), "usr_1"); err != nil {
		t.Fatalf("SubmitForReview: %v", err)
	}
	_, err := h.registration.SubmitForReview(context.Background(), "usr_1")
	if got := errs.CodeOf(err); got != "invalid_status_change" {
		t.Errorf("code = %q, want invalid_status_change", got)
	}
}

func TestSubmittingRecordsWhoSubmitted(t *testing.T) {
	h := newHarness(t)
	merchant := h.register(t, "usr_1", validRequest())
	h.documentAll(t, "usr_1", domain.TypeRestaurant)

	if _, err := h.registration.SubmitForReview(context.Background(), "usr_1"); err != nil {
		t.Fatalf("SubmitForReview: %v", err)
	}
	if len(h.repo.events) != 1 {
		t.Fatalf("events = %d, want 1", len(h.repo.events))
	}
	event := h.repo.events[0]
	if event.MerchantID != merchant.ID || event.From != domain.StatusDraft ||
		event.To != domain.StatusPendingReview || event.ActorUserID != "usr_1" {
		t.Errorf("event = %+v", event)
	}
}

func TestSubmittingReportsAStoreThatIsDown(t *testing.T) {
	for name, broken := range map[string]func(*memoryRepo){
		"save":  func(r *memoryRepo) { r.saveErr = errStore },
		"audit": func(r *memoryRepo) { r.eventErr = errStore },
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			h.register(t, "usr_1", validRequest())
			h.documentAll(t, "usr_1", domain.TypeRestaurant)
			broken(h.repo)

			_, err := h.registration.SubmitForReview(context.Background(), "usr_1")
			if errs.KindOf(err) != errs.KindUnavailable {
				t.Errorf("kind = %v, want unavailable", errs.KindOf(err))
			}
		})
	}
}

// -------------------------------------------------------------- withdrawal

func TestADraftCanBeWithdrawn(t *testing.T) {
	h := newHarness(t)
	merchant := h.register(t, "usr_1", validRequest())

	if err := h.registration.Withdraw(context.Background(), "usr_1"); err != nil {
		t.Fatalf("Withdraw: %v", err)
	}
	if _, ok := h.repo.merchants[merchant.ID]; ok {
		t.Error("the shop record survived the withdrawal")
	}
	if _, ok := h.geo.placed[merchant.ID]; ok {
		t.Error("the shop is still in the search index")
	}
}

func TestARejectedShopCanBeWithdrawn(t *testing.T) {
	h := newHarness(t)
	merchant := h.register(t, "usr_1", validRequest())
	h.documentAll(t, "usr_1", domain.TypeRestaurant)
	if _, err := h.registration.SubmitForReview(context.Background(), "usr_1"); err != nil {
		t.Fatalf("SubmitForReview: %v", err)
	}
	if _, err := h.moderation.Reject(context.Background(), "usr_admin", merchant.ID, "Unreadable licence."); err != nil {
		t.Fatalf("Reject: %v", err)
	}

	if err := h.registration.Withdraw(context.Background(), "usr_1"); err != nil {
		t.Errorf("Withdraw: %v", err)
	}
}

// TestAnApprovedShopCannotBeDeletedByItsOwner: orders, reviews and payouts
// point at a merchant id, and letting an owner delete one because a customer
// complained would take the evidence with it.
func TestAnApprovedShopCannotBeDeletedByItsOwner(t *testing.T) {
	h := newHarness(t)
	h.approve(t, "usr_1")

	err := h.registration.Withdraw(context.Background(), "usr_1")
	if got := errs.CodeOf(err); got != "cannot_withdraw" {
		t.Errorf("code = %q, want cannot_withdraw", got)
	}
}

// TestWithdrawalLeavesTheIndexFirst: the reverse order could leave a point
// pointing at a merchant that no longer exists.
func TestWithdrawalLeavesTheIndexFirst(t *testing.T) {
	h := newHarness(t)
	merchant := h.register(t, "usr_1", validRequest())
	h.geo.removeErr = errs.New(errs.KindUnavailable, "merchant_place_failed", "Try again.")

	if err := h.registration.Withdraw(context.Background(), "usr_1"); err == nil {
		t.Fatal("Withdraw succeeded despite a failed index removal")
	}
	if _, ok := h.repo.merchants[merchant.ID]; !ok {
		t.Error("the record was deleted even though the index still holds it")
	}
}

func TestWithdrawalReportsAStoreThatIsDown(t *testing.T) {
	h := newHarness(t)
	h.register(t, "usr_1", validRequest())
	h.repo.deleteErr = errStore

	if errs.KindOf(h.registration.Withdraw(context.Background(), "usr_1")) != errs.KindUnavailable {
		t.Error("a failed delete was not reported as unavailable")
	}
}

// longString builds a string of n Bengali characters.
func longString(n int) string {
	out := make([]rune, n)
	for i := range out {
		out[i] = 'অ'
	}
	return string(out)
}

// TestRegisteringWithNoOwnerIsRefused covers the guard the transport makes
// unreachable: the owner id comes from a verified token, so a caller can only
// reach this by wiring the use case up wrongly — which is exactly when a clear
// refusal beats a nil merchant.
func TestRegisteringWithNoOwnerIsRefused(t *testing.T) {
	h := newHarness(t)

	_, err := h.registration.Register(context.Background(), "   ", validRequest())
	if got := errs.CodeOf(err); got != "invalid_shop_details" {
		t.Errorf("code = %q, want invalid_shop_details", got)
	}
	if errs.KindOf(err) != errs.KindInvalid {
		t.Errorf("kind = %v, want invalid", errs.KindOf(err))
	}
}
