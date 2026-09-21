package integration

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/identity/domain"
	identitypg "github.com/rootlogic-lab/delivery/backend/internal/modules/identity/infrastructure/persistence/postgres"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/clock"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/id"
)

// withDirectory runs a test inside a transaction that is always rolled back.
func withDirectory(t *testing.T, fn func(ctx context.Context, tx pgx.Tx, dir *identitypg.Directory)) {
	t.Helper()
	ctx := context.Background()
	conn := connect(t)

	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	fn(ctx, tx, identitypg.New(tx, id.NewGen(clock.System{}, nil)))
}

func mustPhone(t *testing.T, raw string) domain.Phone {
	t.Helper()
	p, err := domain.NewPhone(raw)
	if err != nil {
		t.Fatalf("NewPhone(%q): %v", raw, err)
	}
	return p
}

func TestFirstSignInCreatesAnAccount(t *testing.T) {
	withDirectory(t, func(ctx context.Context, _ pgx.Tx, dir *identitypg.Directory) {
		userID, created, err := dir.EnsureUser(ctx, mustPhone(t, "01712345678"), domain.RoleCustomer)
		if err != nil {
			t.Fatalf("EnsureUser: %v", err)
		}
		if !created {
			t.Error("the first sign-in was not reported as new")
		}
		if userID == "" {
			t.Error("no account id was returned")
		}
	})
}

// TestReturningSignInsReuseTheAccount, and report that they are not new so the
// client does not ask for a name again.
func TestReturningSignInsReuseTheAccount(t *testing.T) {
	withDirectory(t, func(ctx context.Context, _ pgx.Tx, dir *identitypg.Directory) {
		phone := mustPhone(t, "01712345678")

		first, created, err := dir.EnsureUser(ctx, phone, domain.RoleCustomer)
		if err != nil || !created {
			t.Fatalf("first sign-in = %v, %v", created, err)
		}
		second, created, err := dir.EnsureUser(ctx, phone, domain.RoleCustomer)
		if err != nil {
			t.Fatalf("second sign-in: %v", err)
		}
		if created {
			t.Error("a returning user was reported as new")
		}
		if second != first {
			t.Errorf("account id changed from %s to %s", first, second)
		}
	})
}

// TestEverySpellingOfANumberReachesOneAccount: the domain normalises before the
// database sees it, so 01712345678 and +8801712345678 cannot become two
// accounts that each get their own allowance.
func TestEverySpellingOfANumberReachesOneAccount(t *testing.T) {
	withDirectory(t, func(ctx context.Context, _ pgx.Tx, dir *identitypg.Directory) {
		first, _, err := dir.EnsureUser(ctx, mustPhone(t, "01712345678"), domain.RoleCustomer)
		if err != nil {
			t.Fatalf("EnsureUser: %v", err)
		}
		for _, spelling := range []string{"+8801712345678", "8801712345678", "017-1234-5678"} {
			got, created, err := dir.EnsureUser(ctx, mustPhone(t, spelling), domain.RoleCustomer)
			if err != nil {
				t.Fatalf("EnsureUser(%q): %v", spelling, err)
			}
			if created || got != first {
				t.Errorf("%q produced account %s (new=%v), want %s", spelling, got, created, first)
			}
		}
	})
}

// One account per (phone, role): the same person may be a customer and a
// delivery partner, and those are different accounts with different permissions.
func TestOnePersonMayHoldSeparateAccountsPerRole(t *testing.T) {
	withDirectory(t, func(ctx context.Context, _ pgx.Tx, dir *identitypg.Directory) {
		phone := mustPhone(t, "01712345678")

		customer, _, err := dir.EnsureUser(ctx, phone, domain.RoleCustomer)
		if err != nil {
			t.Fatalf("customer: %v", err)
		}
		partner, created, err := dir.EnsureUser(ctx, phone, domain.RolePartner)
		if err != nil {
			t.Fatalf("partner: %v", err)
		}
		if !created {
			t.Error("the partner account was not reported as new")
		}
		if partner == customer {
			t.Error("the customer and partner accounts share an id; their permissions would be shared too")
		}
	})
}

// TestConcurrentFirstSignInsProduceOneAccount is why this is a single upsert
// rather than a select followed by an insert: two devices verifying at the same
// moment would both find nothing and both try to create an account, and the
// loser would see a failed sign-in for no reason they could understand.
func TestConcurrentFirstSignInsProduceOneAccount(t *testing.T) {
	ctx := context.Background()
	conn := connect(t)
	phone := mustPhone(t, "01998877665")

	// Not inside one transaction: the race only exists between connections.
	t.Cleanup(func() {
		_, _ = conn.Exec(ctx, `DELETE FROM identity_accounts WHERE phone = $1`, phone.String())
	})

	const racers = 8
	ids := make([]string, racers)
	created := make([]bool, racers)
	errsSeen := make([]error, racers)

	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			own := connect(t)
			dir := identitypg.New(own, id.NewGen(clock.System{}, nil))
			<-start
			ids[i], created[i], errsSeen[i] = dir.EnsureUser(ctx, phone, domain.RoleCustomer)
		}(i)
	}
	close(start)
	wg.Wait()

	newAccounts := 0
	for i := range ids {
		if errsSeen[i] != nil {
			t.Fatalf("racer %d failed: %v", i, errsSeen[i])
		}
		if ids[i] != ids[0] {
			t.Errorf("racer %d got account %s, racer 0 got %s", i, ids[i], ids[0])
		}
		if created[i] {
			newAccounts++
		}
	}
	if newAccounts != 1 {
		t.Errorf("%d racers reported creating the account, want exactly 1", newAccounts)
	}
}

// A suspended account keeps its history and its sessions can still be revoked,
// which deleting the row would make impossible — but it must not be able to
// sign in.
func TestASuspendedAccountCannotSignIn(t *testing.T) {
	withDirectory(t, func(ctx context.Context, tx pgx.Tx, dir *identitypg.Directory) {
		phone := mustPhone(t, "01712345678")
		if _, _, err := dir.EnsureUser(ctx, phone, domain.RoleCustomer); err != nil {
			t.Fatalf("EnsureUser: %v", err)
		}
		if _, err := tx.Exec(ctx,
			`UPDATE identity_accounts SET is_active = FALSE WHERE phone = $1`, phone.String()); err != nil {
			t.Fatalf("suspend: %v", err)
		}

		_, _, err := dir.EnsureUser(ctx, phone, domain.RoleCustomer)
		if !errors.Is(err, identitypg.ErrAccountSuspended) {
			t.Errorf("error = %v, want ErrAccountSuspended", err)
		}
	})
}

// TestPhoneForFindsTheAccountsNumber is notification's own lookup (P14),
// exercised against the real database rather than a fake so the query and
// the domain.Phone round trip are proven together.
func TestPhoneForFindsTheAccountsNumber(t *testing.T) {
	withDirectory(t, func(ctx context.Context, _ pgx.Tx, dir *identitypg.Directory) {
		phone := mustPhone(t, "01712345678")
		userID, _, err := dir.EnsureUser(ctx, phone, domain.RoleCustomer)
		if err != nil {
			t.Fatalf("EnsureUser: %v", err)
		}

		got, found, err := dir.PhoneFor(ctx, userID)
		if err != nil {
			t.Fatalf("PhoneFor: %v", err)
		}
		if !found || got.String() != phone.String() {
			t.Errorf("PhoneFor = %v, %v, want %s, true", got, found, phone.String())
		}
	})
}

func TestPhoneForAnUnknownAccountIsNotFound(t *testing.T) {
	withDirectory(t, func(ctx context.Context, _ pgx.Tx, dir *identitypg.Directory) {
		_, found, err := dir.PhoneFor(ctx, "usr_missing")
		if err != nil || found {
			t.Errorf("PhoneFor(missing) = found=%v, err=%v", found, err)
		}
	})
}

// A suspended account's number must not be handed out — texting someone who
// closed their account is not what this lookup is for.
func TestPhoneForASuspendedAccountIsNotFound(t *testing.T) {
	withDirectory(t, func(ctx context.Context, tx pgx.Tx, dir *identitypg.Directory) {
		phone := mustPhone(t, "01712345678")
		userID, _, err := dir.EnsureUser(ctx, phone, domain.RoleCustomer)
		if err != nil {
			t.Fatalf("EnsureUser: %v", err)
		}
		if _, err := tx.Exec(ctx,
			`UPDATE identity_accounts SET is_active = FALSE WHERE id = $1`, userID); err != nil {
			t.Fatalf("suspend: %v", err)
		}

		_, found, err := dir.PhoneFor(ctx, userID)
		if err != nil || found {
			t.Errorf("PhoneFor(suspended) = found=%v, err=%v", found, err)
		}
	})
}

// A row whose stored phone does not parse can only mean the stored value and
// domain.NewPhone's rules have drifted since EnsureUser wrote it — reachable
// only by writing the row directly, the way this test does.
func TestPhoneForARowThatDoesNotParse(t *testing.T) {
	withDirectory(t, func(ctx context.Context, tx pgx.Tx, dir *identitypg.Directory) {
		if _, err := tx.Exec(ctx, `
			INSERT INTO identity_accounts (id, phone, role) VALUES ('usr_bad', 'not-a-phone', 'customer')`); err != nil {
			t.Fatalf("insert: %v", err)
		}
		if _, _, err := dir.PhoneFor(ctx, "usr_bad"); err == nil {
			t.Error("PhoneFor must fail on a stored value that no longer parses")
		}
	})
}

func TestPhoneForSurfacesAClosedConnection(t *testing.T) {
	ctx := context.Background()
	conn := connect(t)
	if err := conn.Close(ctx); err != nil {
		t.Fatalf("close: %v", err)
	}
	dir := identitypg.New(conn, id.NewGen(clock.System{}, nil))

	if _, _, err := dir.PhoneFor(ctx, "usr_1"); err == nil {
		t.Error("PhoneFor on a closed connection must fail")
	}
}

func TestTheDatabaseRefusesAnUnknownRole(t *testing.T) {
	withDirectory(t, func(ctx context.Context, tx pgx.Tx, _ *identitypg.Directory) {
		_, err := tx.Exec(ctx, `
			INSERT INTO identity_accounts (id, phone, role)
			VALUES ('usr_x', '+8801712345678', 'superuser')`)
		if err == nil {
			t.Error("the database accepted a role outside the four")
		}
	})
}

func TestTheDirectorySurfacesAClosedConnection(t *testing.T) {
	ctx := context.Background()
	conn := connect(t)
	if err := conn.Close(ctx); err != nil {
		t.Fatalf("close: %v", err)
	}
	dir := identitypg.New(conn, id.NewGen(clock.System{}, nil))

	if _, _, err := dir.EnsureUser(ctx, mustPhone(t, "01712345678"), domain.RoleCustomer); err == nil {
		t.Error("EnsureUser on a closed connection must fail")
	}
}

func TestNewFromPoolIsWired(t *testing.T) {
	if dir := identitypg.NewFromPool(nil, id.NewGen(clock.System{}, nil)); dir == nil {
		t.Error("NewFromPool must return a directory")
	}
}
