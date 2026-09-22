package integration

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/user/domain"
	userpg "github.com/rootlogic-lab/delivery/backend/internal/modules/user/infrastructure/persistence/postgres"
)

// withUserRepo runs a test inside a transaction that is always rolled back.
func withUserRepo(t *testing.T, fn func(ctx context.Context, tx pgx.Tx, repo *userpg.Repository)) {
	t.Helper()
	ctx := context.Background()
	conn := connect(t)

	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	fn(ctx, tx, userpg.New(tx, tx))
}

func sampleAddress(t *testing.T, id, userID string, isDefault bool) domain.Address {
	t.Helper()
	pin, err := domain.NewPin(23.7461, 90.3742)
	if err != nil {
		t.Fatalf("NewPin: %v", err)
	}
	address, err := domain.NewAddress(id, userID, "Home", "Ayesha", "01712345678",
		"House 12, Road 7", "Dhanmondi", "Blue gate", pin)
	if err != nil {
		t.Fatalf("NewAddress: %v", err)
	}
	address = address.WithPlacement(domain.Placement{
		AreaCode: "DHK-DHM", AreaName: "Dhanmondi", DistrictCode: "DHK", DivisionCode: "DHA",
	})
	address.IsDefault = isDefault
	return address
}

func TestAProfileRoundTrips(t *testing.T) {
	withUserRepo(t, func(ctx context.Context, _ pgx.Tx, repo *userpg.Repository) {
		profile, err := domain.NewProfile("usr_1", "Ayesha Rahman", "ayesha@example.com", domain.LanguageEnglish)
		if err != nil {
			t.Fatalf("NewProfile: %v", err)
		}
		if err := repo.SaveProfile(ctx, profile); err != nil {
			t.Fatalf("SaveProfile: %v", err)
		}

		got, err := repo.Profile(ctx, "usr_1")
		if err != nil {
			t.Fatalf("Profile: %v", err)
		}
		if got.Name != profile.Name || got.Email != profile.Email || got.Language != profile.Language {
			t.Errorf("profile = %+v, want %+v", got, profile)
		}
	})
}

func TestSavingAProfileTwiceUpdatesIt(t *testing.T) {
	withUserRepo(t, func(ctx context.Context, _ pgx.Tx, repo *userpg.Repository) {
		first, _ := domain.NewProfile("usr_1", "Ayesha", "", domain.LanguageBengali)
		second, _ := domain.NewProfile("usr_1", "Ayesha Rahman", "a@example.com", domain.LanguageEnglish)

		if err := repo.SaveProfile(ctx, first); err != nil {
			t.Fatalf("first save: %v", err)
		}
		if err := repo.SaveProfile(ctx, second); err != nil {
			t.Fatalf("second save: %v", err)
		}

		got, err := repo.Profile(ctx, "usr_1")
		if err != nil {
			t.Fatalf("Profile: %v", err)
		}
		if got.Name != "Ayesha Rahman" || got.Language != domain.LanguageEnglish {
			t.Errorf("profile = %+v", got)
		}
	})
}

func TestAMissingProfileIsReportedAsSuch(t *testing.T) {
	withUserRepo(t, func(ctx context.Context, _ pgx.Tx, repo *userpg.Repository) {
		if _, err := repo.Profile(ctx, "usr_nobody"); !errors.Is(err, domain.ErrProfileNotFound) {
			t.Errorf("error = %v, want ErrProfileNotFound", err)
		}
	})
}

// A language outside the supported set is schema drift, not a reason to lock
// someone out of their own profile.
func TestAStoredLanguageOutsideTheSetFallsBack(t *testing.T) {
	withUserRepo(t, func(ctx context.Context, tx pgx.Tx, repo *userpg.Repository) {
		profile, _ := domain.NewProfile("usr_1", "Ayesha", "", domain.LanguageEnglish)
		if err := repo.SaveProfile(ctx, profile); err != nil {
			t.Fatalf("SaveProfile: %v", err)
		}
		// The CHECK constraint forbids it through normal writes, so this
		// simulates a future language that has since been removed.
		if _, err := tx.Exec(ctx,
			`ALTER TABLE user_profiles DROP CONSTRAINT user_profiles_language_check`); err != nil {
			t.Fatalf("drop constraint: %v", err)
		}
		if _, err := tx.Exec(ctx,
			`UPDATE user_profiles SET language = 'fr' WHERE user_id = 'usr_1'`); err != nil {
			t.Fatalf("update: %v", err)
		}

		got, err := repo.Profile(ctx, "usr_1")
		if err != nil {
			t.Fatalf("a drifted language failed the read: %v", err)
		}
		if got.Language != domain.DefaultLanguage {
			t.Errorf("language = %q, want the default", got.Language)
		}
	})
}

func TestAnAddressRoundTripsIncludingItsPin(t *testing.T) {
	withUserRepo(t, func(ctx context.Context, _ pgx.Tx, repo *userpg.Repository) {
		address := sampleAddress(t, "adr_1", "usr_1", true)
		if err := repo.SaveAddresses(ctx, "usr_1", []domain.Address{address}); err != nil {
			t.Fatalf("SaveAddresses: %v", err)
		}

		got, err := repo.Address(ctx, "usr_1", "adr_1")
		if err != nil {
			t.Fatalf("Address: %v", err)
		}
		if got.Line1 != address.Line1 || got.RecipientPhone != address.RecipientPhone {
			t.Errorf("address = %+v", got)
		}
		if got.Instructions != "Blue gate" {
			t.Errorf("instructions = %q", got.Instructions)
		}
		// The pin survives the round trip through PostGIS to within a metre.
		if diff := got.Pin.Lat - address.Pin.Lat; diff > 1e-6 || diff < -1e-6 {
			t.Errorf("latitude drifted by %v", diff)
		}
		if diff := got.Pin.Lng - address.Pin.Lng; diff > 1e-6 || diff < -1e-6 {
			t.Errorf("longitude drifted by %v", diff)
		}
		if got.Placement.AreaCode != "DHK-DHM" || got.Placement.DivisionCode != "DHA" {
			t.Errorf("placement = %+v", got.Placement)
		}
	})
}

// TestTheDatabaseEnforcesOneDefaultPerUser. The rule lives in the domain and
// again in a partial unique index, so a direct write cannot create a second
// default — which a trigger somebody disables could.
func TestTheDatabaseEnforcesOneDefaultPerUser(t *testing.T) {
	withUserRepo(t, func(ctx context.Context, tx pgx.Tx, repo *userpg.Repository) {
		if err := repo.SaveAddresses(ctx, "usr_1", []domain.Address{
			sampleAddress(t, "adr_1", "usr_1", true),
		}); err != nil {
			t.Fatalf("SaveAddresses: %v", err)
		}

		_, err := tx.Exec(ctx, `
			INSERT INTO user_addresses (
			    id, user_id, recipient_name, recipient_phone, line1, pin,
			    area_code, district_code, division_code, is_default)
			VALUES ('adr_2', 'usr_1', 'A', '017', 'x',
			        ST_SetSRID(ST_MakePoint(90.3, 23.7), 4326)::geography,
			        'DHK-DHM', 'DHK', 'DHA', TRUE)`)
		if err == nil {
			t.Error("the database allowed a second default address")
		}
	})
}

// Two users may each have their own default: the constraint is per user.
func TestTwoUsersMayEachHaveADefault(t *testing.T) {
	withUserRepo(t, func(ctx context.Context, _ pgx.Tx, repo *userpg.Repository) {
		for _, userID := range []string{"usr_1", "usr_2"} {
			if err := repo.SaveAddresses(ctx, userID, []domain.Address{
				sampleAddress(t, "adr_"+userID, userID, true),
			}); err != nil {
				t.Fatalf("%s: %v", userID, err)
			}
		}
		for _, userID := range []string{"usr_1", "usr_2"} {
			if _, err := repo.DefaultAddress(ctx, userID); err != nil {
				t.Errorf("%s has no default: %v", userID, err)
			}
		}
	})
}

// TestMovingTheDefaultNeverLeavesTwo. Saving the whole list in one transaction
// is what makes this possible: a row-at-a-time update would hit the unique
// index mid-way and fail.
func TestMovingTheDefaultNeverLeavesTwo(t *testing.T) {
	withUserRepo(t, func(ctx context.Context, _ pgx.Tx, repo *userpg.Repository) {
		first := sampleAddress(t, "adr_1", "usr_1", true)
		second := sampleAddress(t, "adr_2", "usr_1", false)
		if err := repo.SaveAddresses(ctx, "usr_1", []domain.Address{first, second}); err != nil {
			t.Fatalf("SaveAddresses: %v", err)
		}

		first.IsDefault, second.IsDefault = false, true
		if err := repo.SaveAddresses(ctx, "usr_1", []domain.Address{first, second}); err != nil {
			t.Fatalf("moving the default: %v", err)
		}

		got, err := repo.DefaultAddress(ctx, "usr_1")
		if err != nil {
			t.Fatalf("DefaultAddress: %v", err)
		}
		if got.ID != "adr_2" {
			t.Errorf("default = %s, want adr_2", got.ID)
		}
	})
}

// TestOneUsersAddressIsUnreachableFromAnother: the user id is in the WHERE
// clause, not checked afterwards.
func TestOneUsersAddressIsUnreachableFromAnother(t *testing.T) {
	withUserRepo(t, func(ctx context.Context, _ pgx.Tx, repo *userpg.Repository) {
		if err := repo.SaveAddresses(ctx, "usr_owner", []domain.Address{
			sampleAddress(t, "adr_1", "usr_owner", true),
		}); err != nil {
			t.Fatalf("SaveAddresses: %v", err)
		}

		if _, err := repo.Address(ctx, "usr_attacker", "adr_1"); !errors.Is(err, domain.ErrAddressNotFound) {
			t.Errorf("error = %v, want the address unreachable", err)
		}
		addresses, err := repo.Addresses(ctx, "usr_attacker")
		if err != nil {
			t.Fatalf("Addresses: %v", err)
		}
		if len(addresses) != 0 {
			t.Errorf("the attacker sees %d addresses", len(addresses))
		}
	})
}

func TestAddressesComeBackOldestFirst(t *testing.T) {
	withUserRepo(t, func(ctx context.Context, _ pgx.Tx, repo *userpg.Repository) {
		want := []string{"adr_1", "adr_2", "adr_3"}
		list := make([]domain.Address, 0, len(want))
		for i, id := range want {
			list = append(list, sampleAddress(t, id, "usr_1", i == 0))
		}
		if err := repo.SaveAddresses(ctx, "usr_1", list); err != nil {
			t.Fatalf("SaveAddresses: %v", err)
		}

		got, err := repo.Addresses(ctx, "usr_1")
		if err != nil {
			t.Fatalf("Addresses: %v", err)
		}
		if len(got) != len(want) {
			t.Fatalf("got %d addresses, want %d", len(got), len(want))
		}
		for i := range want {
			if got[i].ID != want[i] {
				t.Errorf("position %d = %s, want %s", i, got[i].ID, want[i])
			}
		}
	})
}

func TestDeletingAnAddressRemovesIt(t *testing.T) {
	withUserRepo(t, func(ctx context.Context, _ pgx.Tx, repo *userpg.Repository) {
		if err := repo.SaveAddresses(ctx, "usr_1", []domain.Address{
			sampleAddress(t, "adr_1", "usr_1", true),
		}); err != nil {
			t.Fatalf("SaveAddresses: %v", err)
		}
		if err := repo.DeleteAddress(ctx, "usr_1", "adr_1"); err != nil {
			t.Fatalf("DeleteAddress: %v", err)
		}
		if _, err := repo.Address(ctx, "usr_1", "adr_1"); !errors.Is(err, domain.ErrAddressNotFound) {
			t.Errorf("error = %v, want it gone", err)
		}
	})
}

func TestNoDefaultAddressIsReportedAsSuch(t *testing.T) {
	withUserRepo(t, func(ctx context.Context, _ pgx.Tx, repo *userpg.Repository) {
		if _, err := repo.DefaultAddress(ctx, "usr_nobody"); !errors.Is(err, domain.ErrNoDefaultAddress) {
			t.Errorf("error = %v, want ErrNoDefaultAddress", err)
		}
	})
}

func TestUpdatingAnAddressOverwritesIt(t *testing.T) {
	withUserRepo(t, func(ctx context.Context, _ pgx.Tx, repo *userpg.Repository) {
		address := sampleAddress(t, "adr_1", "usr_1", true)
		if err := repo.SaveAddresses(ctx, "usr_1", []domain.Address{address}); err != nil {
			t.Fatalf("SaveAddresses: %v", err)
		}

		address.Line1 = "House 99, Road 1"
		address.Instructions = "Ring twice"
		if err := repo.SaveAddresses(ctx, "usr_1", []domain.Address{address}); err != nil {
			t.Fatalf("update: %v", err)
		}

		got, err := repo.Address(ctx, "usr_1", "adr_1")
		if err != nil {
			t.Fatalf("Address: %v", err)
		}
		if got.Line1 != "House 99, Road 1" || got.Instructions != "Ring twice" {
			t.Errorf("address = %+v", got)
		}

		all, _ := repo.Addresses(ctx, "usr_1")
		if len(all) != 1 {
			t.Errorf("an update created a second row: %d addresses", len(all))
		}
	})
}

func TestTheRepositorySurfacesAClosedConnection(t *testing.T) {
	ctx := context.Background()
	conn := connect(t)
	if err := conn.Close(ctx); err != nil {
		t.Fatalf("close: %v", err)
	}
	repo := userpg.New(conn, conn)

	if _, err := repo.Profile(ctx, "usr_1"); err == nil {
		t.Error("Profile on a closed connection must fail")
	}
	profile, _ := domain.NewProfile("usr_1", "A", "", domain.LanguageBengali)
	if err := repo.SaveProfile(ctx, profile); err == nil {
		t.Error("SaveProfile on a closed connection must fail")
	}
	if _, err := repo.Addresses(ctx, "usr_1"); err == nil {
		t.Error("Addresses on a closed connection must fail")
	}
	if _, err := repo.Address(ctx, "usr_1", "adr_1"); err == nil {
		t.Error("Address on a closed connection must fail")
	}
	if _, err := repo.DefaultAddress(ctx, "usr_1"); err == nil {
		t.Error("DefaultAddress on a closed connection must fail")
	}
	if err := repo.SaveAddresses(ctx, "usr_1", nil); err == nil {
		t.Error("SaveAddresses on a closed connection must fail")
	}
	if err := repo.DeleteAddress(ctx, "usr_1", "adr_1"); err == nil {
		t.Error("DeleteAddress on a closed connection must fail")
	}
}

func TestUserNewFromPoolIsWired(t *testing.T) {
	if repo := userpg.NewFromPool(nil); repo == nil {
		t.Error("NewFromPool must return a repository")
	}
}
