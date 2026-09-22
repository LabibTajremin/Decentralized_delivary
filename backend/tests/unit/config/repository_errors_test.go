package config

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/config/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/config/domain"
	cfgpg "github.com/rootlogic-lab/delivery/backend/internal/modules/config/infrastructure/persistence/postgres"
)

// The integration tests prove the SQL is right against real PostgreSQL. These
// drive the failure branches a healthy database never produces: a connection
// dropped mid-scan, a transaction that will not begin, a commit that fails.
// Getting those wrong means a config write that half-applies, which is the one
// outcome the audit log exists to prevent.

var errDB = errors.New("connection reset by peer")

// stubRows is a canned pgx.Rows over string/bool columns.
type stubRows struct {
	rows    [][]any
	idx     int
	scanErr error
	iterErr error
}

func (s *stubRows) Close()                                       {}
func (s *stubRows) Err() error                                   { return s.iterErr }
func (s *stubRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (s *stubRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (s *stubRows) Values() ([]any, error)                       { return nil, nil }
func (s *stubRows) RawValues() [][]byte                          { return nil }
func (s *stubRows) Conn() *pgx.Conn                              { return nil }
func (s *stubRows) TypeMap() *pgtype.Map                         { return pgtype.NewMap() }

func (s *stubRows) Next() bool {
	if s.idx >= len(s.rows) {
		return false
	}
	s.idx++
	return true
}

func (s *stubRows) Scan(dest ...any) error {
	if s.scanErr != nil {
		return s.scanErr
	}
	values := s.rows[s.idx-1]
	if len(values) < len(dest) {
		return errors.New("stub: not enough values")
	}
	for i, d := range dest {
		switch target := d.(type) {
		case *string:
			*target = values[i].(string)
		case *bool:
			*target = values[i].(bool)
		case *time.Time:
			*target = values[i].(time.Time)
		default:
			return errors.New("stub: unsupported destination type")
		}
	}
	return nil
}

// stubDB scripts Query and Exec.
type stubDB struct {
	rows     *stubRows
	queryErr error
	execErr  error
}

func (s *stubDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	if s.queryErr != nil {
		return nil, s.queryErr
	}
	if s.rows == nil {
		return &stubRows{}, nil
	}
	return s.rows, nil
}

func (s *stubDB) QueryRow(context.Context, string, ...any) pgx.Row { return stubRow{} }

func (s *stubDB) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	if s.execErr != nil {
		return pgconn.CommandTag{}, s.execErr
	}
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

type stubRow struct{}

func (stubRow) Scan(...any) error { return pgx.ErrNoRows }

// stubTx is a pgx.Tx that fails where a test asks it to.
type stubTx struct {
	pgx.Tx
	execErr   error
	commitErr error
	rolledBk  bool
	committed bool
}

func (s *stubTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	if s.execErr != nil {
		return pgconn.CommandTag{}, s.execErr
	}
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

func (s *stubTx) Commit(context.Context) error {
	s.committed = true
	return s.commitErr
}

func (s *stubTx) Rollback(context.Context) error {
	s.rolledBk = true
	return nil
}

// stubBeginner hands out a scripted transaction.
type stubBeginner struct {
	tx       *stubTx
	beginErr error
}

func (s *stubBeginner) Begin(context.Context) (pgx.Tx, error) {
	if s.beginErr != nil {
		return nil, s.beginErr
	}
	return s.tx, nil
}

func bg() context.Context { return context.Background() }

var auditTime = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

func someOverride(t *testing.T) domain.Override {
	t.Helper()
	v, err := domain.Money(6000)
	if err != nil {
		t.Fatalf("Money: %v", err)
	}
	return domain.Override{Key: domain.PricingDeliveryBase, Scope: domain.GlobalScope, Value: v}
}

func someChange(t *testing.T) domain.Change {
	t.Helper()
	old, _ := domain.Money(4000)
	next, _ := domain.Money(6000)
	return domain.Change{
		ID: "cfg_1", Key: domain.PricingDeliveryBase, Scope: domain.GlobalScope,
		OldValue: old, NewValue: next, Actor: domain.AdminActor("adm_1"), Reason: "r",
	}
}

func TestOverridesForSurfacesAQueryFailure(t *testing.T) {
	repo := cfgpg.New(&stubDB{queryErr: errDB}, &stubBeginner{})
	if _, err := repo.OverridesFor(bg(), domain.Placement{}); !errors.Is(err, errDB) {
		t.Errorf("error = %v, want the failure wrapped", err)
	}
}

func TestAllOverridesSurfacesAQueryFailure(t *testing.T) {
	repo := cfgpg.New(&stubDB{queryErr: errDB}, &stubBeginner{})
	if _, err := repo.AllOverrides(bg(), domain.GlobalScope); !errors.Is(err, errDB) {
		t.Errorf("error = %v, want the failure wrapped", err)
	}
}

func TestScanningOverridesSurfacesAScanFailure(t *testing.T) {
	db := &stubDB{rows: &stubRows{
		rows:    [][]any{{"pricing.delivery_base", "global", "", "6000", false}},
		scanErr: errDB,
	}}
	if _, err := cfgpg.New(db, &stubBeginner{}).AllOverrides(bg(), domain.GlobalScope); !errors.Is(err, errDB) {
		t.Errorf("error = %v, want the scan failure wrapped", err)
	}
}

// A connection dropped mid-stream yields a short, error-free-looking result.
// Treating that as the complete set would silently apply global defaults to an
// area that has overrides.
func TestScanningOverridesSurfacesAnIterationFailure(t *testing.T) {
	db := &stubDB{rows: &stubRows{iterErr: errDB}}
	if _, err := cfgpg.New(db, &stubBeginner{}).AllOverrides(bg(), domain.GlobalScope); !errors.Is(err, errDB) {
		t.Errorf("error = %v, want the iteration failure wrapped", err)
	}
}

// A row whose scope level or code no longer makes sense is skipped, exactly as
// an unknown key is: one bad row must not stop configuration loading.
func TestRowsWithAnUnusableScopeAreSkipped(t *testing.T) {
	db := &stubDB{rows: &stubRows{rows: [][]any{
		{"pricing.delivery_base", "planet", "mars", "6000", false},
		{"pricing.delivery_base", "area", "", "6000", false},
		{"pricing.delivery_per_km", "global", "", "1000", false},
	}}}
	got, err := cfgpg.New(db, &stubBeginner{}).AllOverrides(bg(), domain.GlobalScope)
	if err != nil {
		t.Fatalf("AllOverrides: %v", err)
	}
	if len(got) != 1 || got[0].Key != domain.PricingDeliveryPerKm {
		t.Errorf("got %+v, want only the usable row", got)
	}
}

func TestSaveOverrideSurfacesABeginFailure(t *testing.T) {
	repo := cfgpg.New(&stubDB{}, &stubBeginner{beginErr: errDB})
	if err := repo.SaveOverride(bg(), someOverride(t), someChange(t)); !errors.Is(err, errDB) {
		t.Errorf("error = %v, want the begin failure wrapped", err)
	}
}

// TestAFailedWriteRollsBack: the whole point of the transaction is that an
// override never outlives its audit entry.
func TestAFailedWriteRollsBack(t *testing.T) {
	tx := &stubTx{execErr: errDB}
	repo := cfgpg.New(&stubDB{}, &stubBeginner{tx: tx})

	if err := repo.SaveOverride(bg(), someOverride(t), someChange(t)); !errors.Is(err, errDB) {
		t.Errorf("error = %v, want the failure wrapped", err)
	}
	if !tx.rolledBk {
		t.Error("a failed write must roll back")
	}
	if tx.committed {
		t.Error("a failed write must not commit")
	}
}

func TestSaveOverrideSurfacesACommitFailure(t *testing.T) {
	tx := &stubTx{commitErr: errDB}
	repo := cfgpg.New(&stubDB{}, &stubBeginner{tx: tx})
	if err := repo.SaveOverride(bg(), someOverride(t), someChange(t)); !errors.Is(err, errDB) {
		t.Errorf("error = %v, want the commit failure wrapped", err)
	}
}

// A transaction already closed by the server is not an error worth reporting:
// the work is done and nothing is pending.
func TestACommitOnAnAlreadyClosedTransactionIsNotAnError(t *testing.T) {
	tx := &stubTx{commitErr: pgx.ErrTxClosed}
	repo := cfgpg.New(&stubDB{}, &stubBeginner{tx: tx})
	if err := repo.SaveOverride(bg(), someOverride(t), someChange(t)); err != nil {
		t.Errorf("error = %v, want an already-closed transaction tolerated", err)
	}
}

func TestDeleteOverrideSurfacesFailures(t *testing.T) {
	beginFail := cfgpg.New(&stubDB{}, &stubBeginner{beginErr: errDB})
	if err := beginFail.DeleteOverride(bg(), domain.PricingDeliveryBase, domain.GlobalScope, someChange(t)); !errors.Is(err, errDB) {
		t.Errorf("begin failure = %v", err)
	}

	tx := &stubTx{execErr: errDB}
	execFail := cfgpg.New(&stubDB{}, &stubBeginner{tx: tx})
	if err := execFail.DeleteOverride(bg(), domain.PricingDeliveryBase, domain.GlobalScope, someChange(t)); !errors.Is(err, errDB) {
		t.Errorf("exec failure = %v", err)
	}
	if !tx.rolledBk {
		t.Error("a failed delete must roll back")
	}
}

func TestChangesSurfacesAQueryFailure(t *testing.T) {
	repo := cfgpg.New(&stubDB{queryErr: errDB}, &stubBeginner{})
	if _, err := repo.Changes(bg(), ports.ChangeFilter{}); !errors.Is(err, errDB) {
		t.Errorf("error = %v, want the failure wrapped", err)
	}
}

func TestChangesSurfacesAScanFailure(t *testing.T) {
	db := &stubDB{rows: &stubRows{rows: [][]any{{"x"}}, scanErr: errDB}}
	if _, err := cfgpg.New(db, &stubBeginner{}).Changes(bg(), ports.ChangeFilter{}); !errors.Is(err, errDB) {
		t.Errorf("error = %v, want the scan failure wrapped", err)
	}
}

func TestChangesSurfacesAnIterationFailure(t *testing.T) {
	db := &stubDB{rows: &stubRows{iterErr: errDB}}
	if _, err := cfgpg.New(db, &stubBeginner{}).Changes(bg(), ports.ChangeFilter{}); !errors.Is(err, errDB) {
		t.Errorf("error = %v, want the iteration failure wrapped", err)
	}
}

// An audit entry is history: it survives its key being removed from the
// registry, reported as it was written rather than dropped for failing today's
// validation.
func TestAnAuditEntryForARemovedKeyIsStillReturned(t *testing.T) {
	db := &stubDB{rows: &stubRows{rows: [][]any{
		{"cfg_1", "pricing.removed_in_2025", "global", "", "100", "200", "admin", "adm_1", "tidy-up", auditTime},
	}}}
	got, err := cfgpg.New(db, &stubBeginner{}).Changes(bg(), ports.ChangeFilter{})
	if err != nil {
		t.Fatalf("Changes: %v", err)
	}
	if len(got) != 1 || got[0].ID != "cfg_1" {
		t.Fatalf("got %+v, want the historical entry kept", got)
	}
}

// A corrupt scope in the audit log is fatal rather than skipped: unlike an
// override, an audit entry nobody can read is a hole in the record, and
// silently dropping it is how a missing change goes unnoticed.
func TestACorruptAuditScopeIsReported(t *testing.T) {
	db := &stubDB{rows: &stubRows{rows: [][]any{
		{"cfg_1", "pricing.delivery_base", "planet", "mars", "100", "200", "admin", "adm_1", "r", auditTime},
	}}}
	if _, err := cfgpg.New(db, &stubBeginner{}).Changes(bg(), ports.ChangeFilter{}); err == nil {
		t.Error("an unreadable audit entry must be reported, not skipped")
	}
}

func TestACorruptAuditScopeCodeIsReported(t *testing.T) {
	db := &stubDB{rows: &stubRows{rows: [][]any{
		{"cfg_1", "pricing.delivery_base", "area", "", "100", "200", "admin", "adm_1", "r", auditTime},
	}}}
	if _, err := cfgpg.New(db, &stubBeginner{}).Changes(bg(), ports.ChangeFilter{}); err == nil {
		t.Error("an audit entry with an impossible scope must be reported")
	}
}

// TestNewFromPoolIsWired guards the production constructor, which only the API
// wiring uses.
func TestNewFromPoolIsWired(t *testing.T) {
	if repo := cfgpg.NewFromPool(nil); repo == nil {
		t.Error("NewFromPool must return a repository")
	}
}
