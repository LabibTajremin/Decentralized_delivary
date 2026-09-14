package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/domain"
	merchantpg "github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/infrastructure/persistence/postgres"
)

// withMerchantRepo runs a test inside a transaction that is always rolled back.
func withMerchantRepo(t *testing.T, fn func(ctx context.Context, repo *merchantpg.Repository)) {
	t.Helper()
	ctx := context.Background()
	conn := connect(t)

	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	fn(ctx, merchantpg.New(tx, tx))
}

// sampleMerchant is a fully placed, fully documented shop.
func sampleMerchant(t *testing.T, id, ownerID string, kind domain.Type) domain.Merchant {
	t.Helper()
	pin, err := domain.NewPin(23.7461, 90.3742)
	if err != nil {
		t.Fatalf("NewPin: %v", err)
	}

	merchant, err := domain.NewMerchant(id, ownerID, domain.Details{
		Name:    "নূরজাহান হোটেল",
		Type:    kind,
		Phone:   "01712345678",
		Email:   "shop@example.com",
		LogoURL: "/static/demo/logo.png",
		Line1:   "House 12, Road 7",
		Line2:   "Level 2",
		Pin:     pin,
	}, time.Date(2026, time.March, 1, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("NewMerchant: %v", err)
	}

	merchant = merchant.WithPlacement(domain.Placement{
		AreaCode: "DHK-DHM", AreaName: "Dhanmondi", DistrictCode: "DHK", DivisionCode: "DHA",
	})
	for _, doc := range domain.RequiredDocuments(kind) {
		document, docErr := domain.NewDocument(doc, "N-"+doc.String(), "/static/demo/doc.png",
			time.Date(2026, time.March, 1, 10, 0, 0, 0, time.UTC))
		if docErr != nil {
			t.Fatalf("NewDocument: %v", docErr)
		}
		merchant = merchant.WithDocument(document)
	}
	return merchant
}

// TestAMerchantRoundTripsThroughPostgres covers every column, because a column
// the repository writes and never reads back is a column that can be wrong for
// a year.
func TestAMerchantRoundTripsThroughPostgres(t *testing.T) {
	withMerchantRepo(t, func(ctx context.Context, repo *merchantpg.Repository) {
		merchant := sampleMerchant(t, "mch_1", "usr_1", domain.TypePharmacy)

		hours, err := domain.NewWeeklyHours(map[time.Weekday][]domain.Window{
			time.Monday: {mustWindow(t, "07:00-11:00"), mustWindow(t, "18:00-23:00")},
			time.Friday: {mustWindow(t, "15:00-24:00")},
		})
		if err != nil {
			t.Fatalf("NewWeeklyHours: %v", err)
		}
		merchant = merchant.WithHours(hours)

		holiday, err := domain.NewHoliday(
			time.Date(2026, time.March, 20, 0, 0, 0, 0, time.UTC), "ঈদের ছুটি",
			time.Date(2026, time.March, 2, 0, 0, 0, 0, time.UTC))
		if err != nil {
			t.Fatalf("NewHoliday: %v", err)
		}
		merchant = merchant.WithHoliday(holiday)

		if err := repo.Create(ctx, merchant); err != nil {
			t.Fatalf("Create: %v", err)
		}

		got, err := repo.Merchant(ctx, "mch_1")
		if err != nil {
			t.Fatalf("Merchant: %v", err)
		}

		if got.Name != merchant.Name || got.Type != merchant.Type || got.Status != merchant.Status {
			t.Errorf("identity = %q/%q/%q", got.Name, got.Type, got.Status)
		}
		if got.Phone != merchant.Phone || got.Email != merchant.Email || got.LogoURL != merchant.LogoURL {
			t.Errorf("contact = %q/%q/%q", got.Phone, got.Email, got.LogoURL)
		}
		if got.Line1 != merchant.Line1 || got.Line2 != merchant.Line2 {
			t.Errorf("address = %q/%q", got.Line1, got.Line2)
		}
		if got.Placement != merchant.Placement {
			t.Errorf("placement = %+v, want %+v", got.Placement, merchant.Placement)
		}
		if !got.CreatedAt.Equal(merchant.CreatedAt) {
			t.Errorf("created_at = %v, want %v", got.CreatedAt, merchant.CreatedAt)
		}

		// The pin survives the round trip through PostGIS, to the precision the
		// geography type keeps.
		if diff := got.Pin.Lat - merchant.Pin.Lat; diff > 1e-9 || diff < -1e-9 {
			t.Errorf("lat = %v, want %v", got.Pin.Lat, merchant.Pin.Lat)
		}
		if diff := got.Pin.Lng - merchant.Pin.Lng; diff > 1e-9 || diff < -1e-9 {
			t.Errorf("lng = %v, want %v", got.Pin.Lng, merchant.Pin.Lng)
		}

		// The schedule comes back with its invariants intact.
		if windows := got.Hours.Windows(time.Monday); len(windows) != 2 || windows[0].String() != "07:00-11:00" {
			t.Errorf("Monday = %v", windows)
		}
		if windows := got.Hours.Windows(time.Friday); len(windows) != 1 || windows[0].String() != "15:00-24:00" {
			t.Errorf("Friday = %v", windows)
		}
		if len(got.Hours.Windows(time.Tuesday)) != 0 {
			t.Error("Tuesday came back open, and it was left closed")
		}

		if !got.Holiday.Active || got.Holiday.Reason != "ঈদের ছুটি" {
			t.Errorf("holiday = %+v", got.Holiday)
		}
		if !got.Holiday.Until.Equal(merchant.Holiday.Until) {
			t.Errorf("holiday until = %v, want %v", got.Holiday.Until, merchant.Holiday.Until)
		}

		if len(got.Documents) != 3 {
			t.Fatalf("documents = %d, want 3", len(got.Documents))
		}
		drug, ok := got.Document(domain.DocDrugLicence)
		if !ok || drug.Number != "N-drug_licence" {
			t.Errorf("drug licence = %+v", drug)
		}
		if !drug.UploadedAt.Equal(time.Date(2026, time.March, 1, 10, 0, 0, 0, time.UTC)) {
			t.Errorf("uploaded_at = %v", drug.UploadedAt)
		}
	})
}

// TestAnIndefiniteHolidayStoresNoDate: NULL rather than a zero timestamp, so a
// read back cannot mistake year 1 for a reopening date.
func TestAnIndefiniteHolidayStoresNoDate(t *testing.T) {
	withMerchantRepo(t, func(ctx context.Context, repo *merchantpg.Repository) {
		merchant := sampleMerchant(t, "mch_1", "usr_1", domain.TypeGrocery)
		holiday, err := domain.NewHoliday(time.Time{}, "Family emergency", time.Now())
		if err != nil {
			t.Fatalf("NewHoliday: %v", err)
		}
		if err := repo.Create(ctx, merchant.WithHoliday(holiday)); err != nil {
			t.Fatalf("Create: %v", err)
		}

		got, err := repo.Merchant(ctx, "mch_1")
		if err != nil {
			t.Fatalf("Merchant: %v", err)
		}
		if !got.Holiday.Active || !got.Holiday.Until.IsZero() {
			t.Errorf("holiday = %+v, want active with no end date", got.Holiday)
		}
	})
}

// TestOneAccountGetsOneShopInTheDatabase: two registrations racing both pass an
// application check, so the constraint has to be real.
func TestOneAccountGetsOneShopInTheDatabase(t *testing.T) {
	withMerchantRepo(t, func(ctx context.Context, repo *merchantpg.Repository) {
		if err := repo.Create(ctx, sampleMerchant(t, "mch_1", "usr_1", domain.TypeGrocery)); err != nil {
			t.Fatalf("first Create: %v", err)
		}
		err := repo.Create(ctx, sampleMerchant(t, "mch_2", "usr_1", domain.TypeGrocery))
		if !errors.Is(err, domain.ErrAlreadyRegistered) {
			t.Errorf("second Create: error = %v, want ErrAlreadyRegistered", err)
		}
	})
}

func TestAMerchantIsFoundByItsOwner(t *testing.T) {
	withMerchantRepo(t, func(ctx context.Context, repo *merchantpg.Repository) {
		if err := repo.Create(ctx, sampleMerchant(t, "mch_1", "usr_1", domain.TypeGrocery)); err != nil {
			t.Fatalf("Create: %v", err)
		}

		got, err := repo.ByOwner(ctx, "usr_1")
		if err != nil {
			t.Fatalf("ByOwner: %v", err)
		}
		if got.ID != "mch_1" {
			t.Errorf("id = %q", got.ID)
		}
		if _, err := repo.ByOwner(ctx, "usr_nobody"); !errors.Is(err, domain.ErrMerchantNotFound) {
			t.Errorf("error = %v, want ErrMerchantNotFound", err)
		}
		if _, err := repo.Merchant(ctx, "mch_nope"); !errors.Is(err, domain.ErrMerchantNotFound) {
			t.Errorf("error = %v, want ErrMerchantNotFound", err)
		}
	})
}

// TestSavingReplacesTheDocumentSetRatherThanAddingToIt: a row left behind from
// before a type change is a licence a reviewer would count as supplied.
func TestSavingReplacesTheDocumentSetRatherThanAddingToIt(t *testing.T) {
	withMerchantRepo(t, func(ctx context.Context, repo *merchantpg.Repository) {
		merchant := sampleMerchant(t, "mch_1", "usr_1", domain.TypePharmacy)
		if err := repo.Create(ctx, merchant); err != nil {
			t.Fatalf("Create: %v", err)
		}

		// The shop turns out to be a grocery, which needs no drug licence.
		details := domain.Details{
			Name: merchant.Name, Type: domain.TypeGrocery, Phone: merchant.Phone,
			Line1: merchant.Line1, Pin: merchant.Pin,
		}
		changed, err := merchant.WithDetails(details)
		if err != nil {
			t.Fatalf("WithDetails: %v", err)
		}
		changed.Documents = nil
		for _, doc := range domain.RequiredDocuments(domain.TypeGrocery) {
			document, docErr := domain.NewDocument(doc, "N-"+doc.String(), "/f.png", time.Now().UTC())
			if docErr != nil {
				t.Fatalf("NewDocument: %v", docErr)
			}
			changed = changed.WithDocument(document)
		}
		if err := repo.Save(ctx, changed); err != nil {
			t.Fatalf("Save: %v", err)
		}

		got, err := repo.Merchant(ctx, "mch_1")
		if err != nil {
			t.Fatalf("Merchant: %v", err)
		}
		if len(got.Documents) != 2 {
			t.Errorf("documents = %d, want the drug licence pruned", len(got.Documents))
		}
		if _, ok := got.Document(domain.DocDrugLicence); ok {
			t.Error("the drug licence survived the change of shop type")
		}
	})
}

// TestReUploadingADocumentOverwritesTheRow exercises the ON CONFLICT path on
// (merchant_id, kind).
func TestReUploadingADocumentOverwritesTheRow(t *testing.T) {
	withMerchantRepo(t, func(ctx context.Context, repo *merchantpg.Repository) {
		merchant := sampleMerchant(t, "mch_1", "usr_1", domain.TypeGrocery)
		if err := repo.Create(ctx, merchant); err != nil {
			t.Fatalf("Create: %v", err)
		}

		replacement, err := domain.NewDocument(domain.DocTradeLicence, "TRAD-NEW", "/clear.png", time.Now().UTC())
		if err != nil {
			t.Fatalf("NewDocument: %v", err)
		}
		if err := repo.Save(ctx, merchant.WithDocument(replacement)); err != nil {
			t.Fatalf("Save: %v", err)
		}

		got, err := repo.Merchant(ctx, "mch_1")
		if err != nil {
			t.Fatalf("Merchant: %v", err)
		}
		if len(got.Documents) != 2 {
			t.Fatalf("documents = %d, want 2", len(got.Documents))
		}
		stored, ok := got.Document(domain.DocTradeLicence)
		if !ok || stored.Number != "TRAD-NEW" {
			t.Errorf("trade licence = %+v", stored)
		}
	})
}

// TestDeletingAMerchantTakesItsDocumentsAndHistoryWithIt checks the cascade,
// because an orphaned licence number is personal data nobody is looking after.
func TestDeletingAMerchantTakesItsDocumentsAndHistoryWithIt(t *testing.T) {
	withMerchantRepo(t, func(ctx context.Context, repo *merchantpg.Repository) {
		merchant := sampleMerchant(t, "mch_1", "usr_1", domain.TypeGrocery)
		if err := repo.Create(ctx, merchant); err != nil {
			t.Fatalf("Create: %v", err)
		}
		if err := repo.RecordStatusChange(ctx, ports.StatusChange{
			ID: "mse_1", MerchantID: "mch_1", From: domain.StatusDraft,
			To: domain.StatusPendingReview, ActorUserID: "usr_1", At: time.Now().UTC(),
		}); err != nil {
			t.Fatalf("RecordStatusChange: %v", err)
		}

		if err := repo.Delete(ctx, "mch_1"); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		if _, err := repo.Merchant(ctx, "mch_1"); !errors.Is(err, domain.ErrMerchantNotFound) {
			t.Errorf("error = %v, want ErrMerchantNotFound", err)
		}

		events, err := repo.StatusHistory(ctx, "mch_1", 10)
		if err != nil {
			t.Fatalf("StatusHistory: %v", err)
		}
		if len(events) != 0 {
			t.Errorf("history = %d entries, want the cascade to have removed them", len(events))
		}
	})
}

func TestTheApprovalQueueFiltersAndPagesInSQL(t *testing.T) {
	withMerchantRepo(t, func(ctx context.Context, repo *merchantpg.Repository) {
		shops := []struct {
			id, owner string
			kind      domain.Type
			status    domain.Status
			division  string
			created   time.Time
		}{
			{"mch_1", "usr_1", domain.TypeRestaurant, domain.StatusApproved, "DHA", time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)},
			{"mch_2", "usr_2", domain.TypeGrocery, domain.StatusPendingReview, "DHA", time.Date(2026, time.March, 2, 0, 0, 0, 0, time.UTC)},
			{"mch_3", "usr_3", domain.TypeGrocery, domain.StatusPendingReview, "RAJ", time.Date(2026, time.March, 3, 0, 0, 0, 0, time.UTC)},
		}
		for _, s := range shops {
			merchant := sampleMerchant(t, s.id, s.owner, s.kind)
			merchant.Status = s.status
			merchant.CreatedAt = s.created
			merchant = merchant.WithPlacement(domain.Placement{
				AreaCode: "DHK-DHM", AreaName: "Dhanmondi", DistrictCode: "DHK", DivisionCode: s.division,
			})
			if err := repo.Create(ctx, merchant); err != nil {
				t.Fatalf("Create %s: %v", s.id, err)
			}
		}

		cases := map[string]struct {
			filter ports.Filter
			want   []string
		}{
			"everything, newest first": {ports.Filter{Limit: 10}, []string{"mch_3", "mch_2", "mch_1"}},
			"pending only":             {ports.Filter{Status: domain.StatusPendingReview, Limit: 10}, []string{"mch_3", "mch_2"}},
			"groceries":                {ports.Filter{Type: domain.TypeGrocery, Limit: 10}, []string{"mch_3", "mch_2"}},
			"one division":             {ports.Filter{DivisionCode: "RAJ", Limit: 10}, []string{"mch_3"}},
			"all three clauses":        {ports.Filter{Status: domain.StatusPendingReview, Type: domain.TypeGrocery, DivisionCode: "DHA", Limit: 10}, []string{"mch_2"}},
			"first page":               {ports.Filter{Limit: 2}, []string{"mch_3", "mch_2"}},
			"second page":              {ports.Filter{Limit: 2, Offset: 2}, []string{"mch_1"}},
			"past the end":             {ports.Filter{Limit: 2, Offset: 10}, nil},
		}

		for name, tc := range cases {
			t.Run(name, func(t *testing.T) {
				found, err := repo.List(ctx, tc.filter)
				if err != nil {
					t.Fatalf("List: %v", err)
				}
				if len(found) != len(tc.want) {
					t.Fatalf("found %d shops, want %d", len(found), len(tc.want))
				}
				for i := range tc.want {
					if found[i].ID != tc.want[i] {
						t.Errorf("position %d = %q, want %q", i, found[i].ID, tc.want[i])
					}
				}
			})
		}

		// A listing carries each shop's documents, because the reviewer's list
		// is where they decide what to open.
		found, err := repo.List(ctx, ports.Filter{Limit: 10})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		for _, merchant := range found {
			if len(merchant.Documents) == 0 {
				t.Errorf("%s came back with no documents", merchant.ID)
			}
		}
	})
}

func TestTheDecisionHistoryIsNewestFirstAndLimited(t *testing.T) {
	withMerchantRepo(t, func(ctx context.Context, repo *merchantpg.Repository) {
		if err := repo.Create(ctx, sampleMerchant(t, "mch_1", "usr_1", domain.TypeGrocery)); err != nil {
			t.Fatalf("Create: %v", err)
		}

		base := time.Date(2026, time.March, 1, 12, 0, 0, 0, time.UTC)
		decisions := []ports.StatusChange{
			{ID: "mse_1", From: domain.StatusDraft, To: domain.StatusPendingReview, ActorUserID: "usr_1", At: base},
			{ID: "mse_2", From: domain.StatusPendingReview, To: domain.StatusRejected, ActorUserID: "usr_admin", Note: "Unreadable.", At: base.Add(time.Hour)},
			{ID: "mse_3", From: domain.StatusRejected, To: domain.StatusPendingReview, ActorUserID: "usr_1", At: base.Add(2 * time.Hour)},
		}
		for _, decision := range decisions {
			decision.MerchantID = "mch_1"
			if err := repo.RecordStatusChange(ctx, decision); err != nil {
				t.Fatalf("RecordStatusChange %s: %v", decision.ID, err)
			}
		}

		events, err := repo.StatusHistory(ctx, "mch_1", 10)
		if err != nil {
			t.Fatalf("StatusHistory: %v", err)
		}
		if len(events) != 3 || events[0].ID != "mse_3" || events[2].ID != "mse_1" {
			t.Fatalf("events = %+v, want newest first", events)
		}
		if events[1].Note != "Unreadable." || events[1].ActorUserID != "usr_admin" {
			t.Errorf("middle event = %+v", events[1])
		}
		if !events[0].At.Equal(base.Add(2 * time.Hour)) {
			t.Errorf("at = %v", events[0].At)
		}

		limited, err := repo.StatusHistory(ctx, "mch_1", 2)
		if err != nil {
			t.Fatalf("StatusHistory: %v", err)
		}
		if len(limited) != 2 {
			t.Errorf("limited = %d entries, want 2", len(limited))
		}
	})
}

// TestAShopWithNoDocumentsSavesCleanly: the prune statement runs with an empty
// list, which is the ANY($2) edge case a non-empty set would never exercise.
func TestAShopWithNoDocumentsSavesCleanly(t *testing.T) {
	withMerchantRepo(t, func(ctx context.Context, repo *merchantpg.Repository) {
		merchant := sampleMerchant(t, "mch_1", "usr_1", domain.TypeGrocery)
		merchant.Documents = nil

		if err := repo.Create(ctx, merchant); err != nil {
			t.Fatalf("Create: %v", err)
		}
		got, err := repo.Merchant(ctx, "mch_1")
		if err != nil {
			t.Fatalf("Merchant: %v", err)
		}
		if len(got.Documents) != 0 {
			t.Errorf("documents = %d, want none", len(got.Documents))
		}
	})
}

// mustWindow parses an opening window or fails the test.
func mustWindow(t *testing.T, s string) domain.Window {
	t.Helper()
	w, err := domain.ParseWindow(s)
	if err != nil {
		t.Fatalf("ParseWindow(%q): %v", s, err)
	}
	return w
}
