package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/config/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/config/domain"
	cfgpg "github.com/rootlogic-lab/delivery/backend/internal/modules/config/infrastructure/persistence/postgres"
)

// These run against real PostgreSQL. The unit tests prove the resolution rule;
// these prove the SQL that feeds it — the four-scope query, the upsert, and
// that an override and its audit entry really are written atomically.

// withConfigTx runs a test inside a transaction that is always rolled back, so
// tests share one schema without sharing state.
func withConfigTx(t *testing.T, fn func(ctx context.Context, tx pgx.Tx, repo *cfgpg.Repository)) {
	t.Helper()
	ctx := context.Background()
	conn := connect(t)

	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// The repository begins its own transactions for writes. Handing it the
	// outer transaction as the beginner makes those savepoints inside it, so
	// the rollback still undoes everything.
	fn(ctx, tx, cfgpg.New(tx, tx))
}

func mustScope(t *testing.T, level domain.Level, code string) domain.Scope {
	t.Helper()
	s, err := domain.NewScope(level, code)
	if err != nil {
		t.Fatalf("NewScope: %v", err)
	}
	return s
}

func money(t *testing.T, minor int64) domain.Value {
	t.Helper()
	v, err := domain.Money(minor)
	if err != nil {
		t.Fatalf("Money: %v", err)
	}
	return v
}

func change(t *testing.T, id string, key domain.Key, scope domain.Scope, from, to domain.Value) domain.Change {
	t.Helper()
	return domain.Change{
		ID: id, Key: key, Scope: scope,
		OldValue: from, NewValue: to,
		Actor:  domain.AdminActor("adm_1"),
		Reason: "integration test",
		At:     time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
	}
}

func TestSaveOverrideWritesTheValueAndTheAuditEntry(t *testing.T) {
	withConfigTx(t, func(ctx context.Context, _ pgx.Tx, repo *cfgpg.Repository) {
		scope := mustScope(t, domain.LevelArea, "DHK-DHM")
		value := money(t, 6000)

		err := repo.SaveOverride(ctx,
			domain.Override{Key: domain.PricingDeliveryBase, Scope: scope, Value: value},
			change(t, "cfg_1", domain.PricingDeliveryBase, scope, money(t, 4000), value))
		if err != nil {
			t.Fatalf("SaveOverride: %v", err)
		}

		stored, err := repo.AllOverrides(ctx, scope)
		if err != nil {
			t.Fatalf("AllOverrides: %v", err)
		}
		if len(stored) != 1 || !stored[0].Value.Equal(value) {
			t.Fatalf("stored = %+v", stored)
		}

		changes, err := repo.Changes(ctx, ports.ChangeFilter{})
		if err != nil {
			t.Fatalf("Changes: %v", err)
		}
		if len(changes) != 1 || changes[0].ID != "cfg_1" {
			t.Fatalf("audit = %+v", changes)
		}
		if changes[0].OldValue.String() != "4000" || changes[0].NewValue.String() != "6000" {
			t.Errorf("audit values = %s -> %s", changes[0].OldValue, changes[0].NewValue)
		}
		if changes[0].Actor.Kind != domain.ActorAdmin || changes[0].Actor.ID != "adm_1" {
			t.Errorf("audit actor = %+v", changes[0].Actor)
		}
	})
}

// TestTheAuditEntryAndTheOverrideAreAtomic: a change that is applied but not
// recorded is exactly the change someone will need to explain later.
func TestTheAuditEntryAndTheOverrideAreAtomic(t *testing.T) {
	withConfigTx(t, func(ctx context.Context, _ pgx.Tx, repo *cfgpg.Repository) {
		scope := mustScope(t, domain.LevelArea, "DHK-DHM")
		value := money(t, 6000)

		// A change whose id is too long for the column would fail the audit
		// insert after the override insert has already succeeded.
		bad := change(t, "cfg_1", domain.PricingDeliveryBase, scope, money(t, 4000), value)
		bad.Actor.Kind = "not_a_real_actor" // violates the CHECK constraint

		if err := repo.SaveOverride(ctx,
			domain.Override{Key: domain.PricingDeliveryBase, Scope: scope, Value: value},
			bad); err == nil {
			t.Fatal("a bad audit entry must fail the whole write")
		}

		stored, err := repo.AllOverrides(ctx, scope)
		if err != nil {
			t.Fatalf("AllOverrides: %v", err)
		}
		if len(stored) != 0 {
			t.Errorf("the override survived a failed audit write: %+v", stored)
		}
	})
}

// TestOverridesForReadsEveryApplicableScopeInOneQuery is the hot path: it runs
// on every pricing and discovery request.
func TestOverridesForReadsEveryApplicableScope(t *testing.T) {
	withConfigTx(t, func(ctx context.Context, _ pgx.Tx, repo *cfgpg.Repository) {
		seed := []struct {
			level domain.Level
			code  string
			minor int64
		}{
			{domain.LevelGlobal, "", 1000},
			{domain.LevelDivision, "DHA", 2000},
			{domain.LevelDistrict, "DHK", 3000},
			{domain.LevelArea, "DHK-DHM", 4000},
			{domain.LevelArea, "DHK-GUL", 9999}, // another area entirely
		}
		for i, s := range seed {
			scope := mustScope(t, s.level, s.code)
			value := money(t, s.minor)
			if err := repo.SaveOverride(ctx,
				domain.Override{Key: domain.PricingDeliveryBase, Scope: scope, Value: value},
				change(t, "cfg_"+string(rune('a'+i)), domain.PricingDeliveryBase, scope, money(t, 4000), value)); err != nil {
				t.Fatalf("seed %d: %v", i, err)
			}
		}

		placement := domain.Placement{AreaCode: "DHK-DHM", DistrictCode: "DHK", DivisionCode: "DHA"}
		got, err := repo.OverridesFor(ctx, placement)
		if err != nil {
			t.Fatalf("OverridesFor: %v", err)
		}
		if len(got) != 4 {
			t.Fatalf("loaded %d overrides, want 4 — the other area must not be included: %+v", len(got), got)
		}

		// And the resolution rule applied to them picks the area's value.
		resolved := domain.Resolve(placement, got)
		effective, err := resolved.Int(domain.PricingDeliveryBase)
		if err != nil {
			t.Fatalf("Int: %v", err)
		}
		if effective != 4000 {
			t.Errorf("effective = %d, want the area value 4000", effective)
		}
	})
}

func TestSaveOverrideUpsertsRatherThanDuplicating(t *testing.T) {
	withConfigTx(t, func(ctx context.Context, _ pgx.Tx, repo *cfgpg.Repository) {
		scope := mustScope(t, domain.LevelArea, "DHK-DHM")

		for i, minor := range []int64{6000, 7000} {
			value := money(t, minor)
			if err := repo.SaveOverride(ctx,
				domain.Override{Key: domain.PricingDeliveryBase, Scope: scope, Value: value, Pinned: i == 1},
				change(t, "cfg_"+string(rune('a'+i)), domain.PricingDeliveryBase, scope, money(t, 4000), value)); err != nil {
				t.Fatalf("save %d: %v", i, err)
			}
		}

		stored, err := repo.AllOverrides(ctx, scope)
		if err != nil {
			t.Fatalf("AllOverrides: %v", err)
		}
		if len(stored) != 1 {
			t.Fatalf("stored %d rows, want 1 upserted row", len(stored))
		}
		if stored[0].Value.String() != "7000" || !stored[0].Pinned {
			t.Errorf("stored = %+v, want the latest value and pin", stored[0])
		}

		// Both writes are still in the audit log: history is append-only.
		changes, err := repo.Changes(ctx, ports.ChangeFilter{})
		if err != nil {
			t.Fatalf("Changes: %v", err)
		}
		if len(changes) != 2 {
			t.Errorf("audit entries = %d, want both writes recorded", len(changes))
		}
	})
}

func TestDeleteOverrideRemovesItAndRecordsWhy(t *testing.T) {
	withConfigTx(t, func(ctx context.Context, _ pgx.Tx, repo *cfgpg.Repository) {
		scope := mustScope(t, domain.LevelArea, "DHK-DHM")
		value := money(t, 6000)
		if err := repo.SaveOverride(ctx,
			domain.Override{Key: domain.PricingDeliveryBase, Scope: scope, Value: value},
			change(t, "cfg_a", domain.PricingDeliveryBase, scope, money(t, 4000), value)); err != nil {
			t.Fatalf("SaveOverride: %v", err)
		}

		if err := repo.DeleteOverride(ctx, domain.PricingDeliveryBase, scope,
			change(t, "cfg_b", domain.PricingDeliveryBase, scope, value, money(t, 4000))); err != nil {
			t.Fatalf("DeleteOverride: %v", err)
		}

		stored, err := repo.AllOverrides(ctx, scope)
		if err != nil {
			t.Fatalf("AllOverrides: %v", err)
		}
		if len(stored) != 0 {
			t.Errorf("stored = %+v, want it removed", stored)
		}

		changes, err := repo.Changes(ctx, ports.ChangeFilter{})
		if err != nil {
			t.Fatalf("Changes: %v", err)
		}
		if len(changes) != 2 {
			t.Errorf("audit entries = %d, want the removal recorded too", len(changes))
		}
	})
}

// TestTheAuditLogSurvivesTheOverrideBeingDeleted: "why did this area go back to
// the default" is the question the log has to answer, and deriving history from
// the overrides table would lose it.
func TestTheAuditLogSurvivesTheOverrideBeingDeleted(t *testing.T) {
	withConfigTx(t, func(ctx context.Context, _ pgx.Tx, repo *cfgpg.Repository) {
		scope := mustScope(t, domain.LevelArea, "DHK-DHM")
		value := money(t, 6000)
		_ = repo.SaveOverride(ctx,
			domain.Override{Key: domain.PricingDeliveryBase, Scope: scope, Value: value},
			change(t, "cfg_a", domain.PricingDeliveryBase, scope, money(t, 4000), value))
		_ = repo.DeleteOverride(ctx, domain.PricingDeliveryBase, scope,
			change(t, "cfg_b", domain.PricingDeliveryBase, scope, value, money(t, 4000)))

		changes, err := repo.Changes(ctx, ports.ChangeFilter{Key: domain.PricingDeliveryBase})
		if err != nil {
			t.Fatalf("Changes: %v", err)
		}
		if len(changes) != 2 {
			t.Fatalf("audit entries = %d, want both", len(changes))
		}
		for _, c := range changes {
			if c.Reason == "" {
				t.Errorf("audit entry %s has no reason", c.ID)
			}
		}
	})
}

func TestChangesCanBeFilteredByScope(t *testing.T) {
	withConfigTx(t, func(ctx context.Context, _ pgx.Tx, repo *cfgpg.Repository) {
		area := mustScope(t, domain.LevelArea, "DHK-DHM")
		global := domain.GlobalScope
		value := money(t, 6000)

		_ = repo.SaveOverride(ctx, domain.Override{Key: domain.PricingDeliveryBase, Scope: area, Value: value},
			change(t, "cfg_a", domain.PricingDeliveryBase, area, money(t, 4000), value))
		_ = repo.SaveOverride(ctx, domain.Override{Key: domain.PricingDeliveryBase, Scope: global, Value: value},
			change(t, "cfg_b", domain.PricingDeliveryBase, global, money(t, 4000), value))

		got, err := repo.Changes(ctx, ports.ChangeFilter{Scope: &area})
		if err != nil {
			t.Fatalf("Changes: %v", err)
		}
		if len(got) != 1 || got[0].ID != "cfg_a" {
			t.Errorf("filtered audit = %+v, want only the area change", got)
		}
	})
}

func TestChangesRespectsALimit(t *testing.T) {
	withConfigTx(t, func(ctx context.Context, _ pgx.Tx, repo *cfgpg.Repository) {
		scope := domain.GlobalScope
		for i := 0; i < 5; i++ {
			value := money(t, int64(4000+i*100))
			_ = repo.SaveOverride(ctx, domain.Override{Key: domain.PricingDeliveryBase, Scope: scope, Value: value},
				change(t, "cfg_"+string(rune('a'+i)), domain.PricingDeliveryBase, scope, money(t, 4000), value))
		}
		got, err := repo.Changes(ctx, ports.ChangeFilter{Limit: 2})
		if err != nil {
			t.Fatalf("Changes: %v", err)
		}
		if len(got) != 2 {
			t.Errorf("got %d changes, want the limit respected", len(got))
		}
	})
}

// TestAStaleRowDoesNotBreakConfigLoading: a key removed from the registry
// leaves rows behind, and one of them must not take the whole config down.
func TestAStaleRowDoesNotBreakConfigLoading(t *testing.T) {
	withConfigTx(t, func(ctx context.Context, tx pgx.Tx, repo *cfgpg.Repository) {
		if _, err := tx.Exec(ctx, `
			INSERT INTO config_overrides (config_key, scope_level, scope_code, value)
			VALUES ('pricing.removed_in_2025', 'global', '', '999')`); err != nil {
			t.Fatalf("seed stale row: %v", err)
		}
		// And a row whose stored text no longer parses as its declared type.
		if _, err := tx.Exec(ctx, `
			INSERT INTO config_overrides (config_key, scope_level, scope_code, value)
			VALUES ('pricing.delivery_per_km', 'global', '', 'not a number')`); err != nil {
			t.Fatalf("seed unparseable row: %v", err)
		}

		got, err := repo.OverridesFor(ctx, domain.Placement{DivisionCode: "DHA"})
		if err != nil {
			t.Fatalf("OverridesFor must not fail on a stale row: %v", err)
		}
		for _, o := range got {
			if o.Key == "pricing.removed_in_2025" {
				t.Error("a removed key was returned")
			}
			if o.Key == domain.PricingDeliveryPerKm {
				t.Error("an unparseable value was returned")
			}
		}
	})
}

// TestTheGlobalScopeCannotHaveACode is enforced by the database as well as the
// domain, so a direct insert cannot create a global row that shadows another.
func TestTheGlobalScopeCannotHaveACode(t *testing.T) {
	withConfigTx(t, func(ctx context.Context, tx pgx.Tx, _ *cfgpg.Repository) {
		_, err := tx.Exec(ctx, `
			INSERT INTO config_overrides (config_key, scope_level, scope_code, value)
			VALUES ('pricing.delivery_base', 'global', 'DHA', '6000')`)
		if err == nil {
			t.Error("the database allowed a global override with a scope code")
		}
	})
}

// A non-global scope with no code is refused for the same reason: it would
// match nothing and silently do nothing.
func TestANonGlobalScopeMustHaveACode(t *testing.T) {
	withConfigTx(t, func(ctx context.Context, tx pgx.Tx, _ *cfgpg.Repository) {
		_, err := tx.Exec(ctx, `
			INSERT INTO config_overrides (config_key, scope_level, scope_code, value)
			VALUES ('pricing.delivery_base', 'area', '', '6000')`)
		if err == nil {
			t.Error("the database allowed an area override with no code")
		}
	})
}

func TestAnUnknownActorKindIsRefusedByTheDatabase(t *testing.T) {
	withConfigTx(t, func(ctx context.Context, tx pgx.Tx, _ *cfgpg.Repository) {
		_, err := tx.Exec(ctx, `
			INSERT INTO config_changes
			    (id, config_key, scope_level, scope_code, old_value, new_value, actor_kind, reason)
			VALUES ('cfg_x', 'pricing.delivery_base', 'global', '', '4000', '6000', 'hacker', 'r')`)
		if err == nil {
			t.Error("the database allowed an unrecognised actor kind")
		}
	})
}

func TestRepositorySurfacesAClosedConnection(t *testing.T) {
	ctx := context.Background()
	conn := connect(t)
	if err := conn.Close(ctx); err != nil {
		t.Fatalf("close: %v", err)
	}
	repo := cfgpg.New(conn, conn)

	if _, err := repo.OverridesFor(ctx, domain.Placement{}); err == nil {
		t.Error("OverridesFor on a closed connection must fail")
	}
	if _, err := repo.AllOverrides(ctx, domain.GlobalScope); err == nil {
		t.Error("AllOverrides on a closed connection must fail")
	}
	if _, err := repo.Changes(ctx, ports.ChangeFilter{}); err == nil {
		t.Error("Changes on a closed connection must fail")
	}
	err := repo.SaveOverride(ctx,
		domain.Override{Key: domain.PricingDeliveryBase, Scope: domain.GlobalScope, Value: money(t, 6000)},
		change(t, "cfg_x", domain.PricingDeliveryBase, domain.GlobalScope, money(t, 4000), money(t, 6000)))
	if err == nil || !errors.Is(err, err) {
		t.Error("SaveOverride on a closed connection must fail")
	}
	if err := repo.DeleteOverride(ctx, domain.PricingDeliveryBase, domain.GlobalScope,
		change(t, "cfg_y", domain.PricingDeliveryBase, domain.GlobalScope, money(t, 6000), money(t, 4000))); err == nil {
		t.Error("DeleteOverride on a closed connection must fail")
	}
}
