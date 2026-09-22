package config

import (
	"errors"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/config/application"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/config/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/config/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

func newTuner(repo *fakeRepo) *application.AutoTuneUseCase {
	return application.NewAutoTuneUseCase(application.NewResolveUseCase(repo), newSetter(repo))
}

// A tuning pass over an area with thin merchant density widens both radii
// ALG-09 governs, each recorded in the audit log with the tuner as the
// actor.
func TestAutoTuneWidensBothRadiiWhenThin(t *testing.T) {
	repo := &fakeRepo{}
	outcomes, err := newTuner(repo).Execute(ctx(), dhanmondi, domain.TuningSignal{MerchantsNearby: 1})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(outcomes) != 2 {
		t.Fatalf("outcomes = %+v, want one per tunable key", outcomes)
	}
	for _, o := range outcomes {
		if !o.Applied {
			t.Errorf("%s was not applied: %+v", o.Key, o)
		}
		if o.Change.Actor.Kind != domain.ActorAutoTuner {
			t.Errorf("%s actor = %+v, want the tuner", o.Key, o.Change.Actor)
		}
		if o.Change.Reason == "" {
			t.Errorf("%s has no reason recorded", o.Key)
		}
	}
	if len(repo.changes) != 2 {
		t.Errorf("audit entries = %d, want 2", len(repo.changes))
	}
}

// A comfortable area is left alone, and the pass says so rather than
// silently doing nothing.
func TestAutoTuneReportsNoChangeNeeded(t *testing.T) {
	repo := &fakeRepo{}
	outcomes, err := newTuner(repo).Execute(ctx(), dhanmondi, domain.TuningSignal{MerchantsNearby: 5, OrderFailureRate: 0.02})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	for _, o := range outcomes {
		if o.Applied || o.Skipped != "no_change_needed" {
			t.Errorf("%s = %+v, want no_change_needed", o.Key, o)
		}
	}
	if len(repo.changes) != 0 {
		t.Errorf("audit entries = %d, want none", len(repo.changes))
	}
}

// An admin's pin stops the tuner touching that one variable, and the pass
// reports why without failing the whole run.
func TestAutoTuneRespectsAPin(t *testing.T) {
	scope := areaScope(t)
	pinned := domain.Override{Key: domain.DiscoveryBaseRadius, Scope: scope, Pinned: true}
	pinned.Value, _ = domain.Distance(5000)
	repo := &fakeRepo{overrides: []domain.Override{pinned}}

	outcomes, err := newTuner(repo).Execute(ctx(), dhanmondi, domain.TuningSignal{MerchantsNearby: 1})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var base, partner *application.TuneOutcome
	for i := range outcomes {
		switch outcomes[i].Key {
		case domain.DiscoveryBaseRadius:
			base = &outcomes[i]
		case domain.DispatchPartnerRadius:
			partner = &outcomes[i]
		}
	}
	if base == nil || base.Applied || base.Skipped != "pinned" {
		t.Errorf("base radius = %+v, want pinned", base)
	}
	if partner == nil || !partner.Applied {
		t.Errorf("partner radius = %+v, want applied — the pin is scoped to one key", partner)
	}
}

func TestAutoTuneRejectsAnInvalidArea(t *testing.T) {
	repo := &fakeRepo{}
	if _, err := newTuner(repo).Execute(ctx(), domain.Placement{}, domain.TuningSignal{}); errs.CodeOf(err) != "invalid_scope" {
		t.Fatalf("err = %v", err)
	}
}

func TestAutoTuneSurfacesAResolveFailure(t *testing.T) {
	repo := &fakeRepo{loadErr: errors.New("boom")}
	if _, err := newTuner(repo).Execute(ctx(), dhanmondi, domain.TuningSignal{}); errs.CodeOf(err) != "config_unavailable" {
		t.Fatalf("err = %v", err)
	}
}

// A write failure genuinely unrelated to a pin — a storage outage — stops
// the pass and is returned, not swallowed the way a pin is.
func TestAutoTuneSurfacesAWriteFailure(t *testing.T) {
	repo := &fakeRepo{writeErr: errors.New("boom")}
	if _, err := newTuner(repo).Execute(ctx(), dhanmondi, domain.TuningSignal{MerchantsNearby: 1}); errs.CodeOf(err) != "config_write_failed" {
		t.Fatalf("err = %v", err)
	}
}

// ListChangesUseCase

func newLister(repo *fakeRepo) *application.ListChangesUseCase {
	return application.NewListChangesUseCase(repo)
}

func TestListChangesReturnsTheAuditLog(t *testing.T) {
	repo := &fakeRepo{}
	if _, err := newSetter(repo).Execute(ctx(), application.SetRequest{
		Key: domain.PricingDeliveryBase, Scope: areaScope(t), Value: mustMoney(t, 6000),
		Actor: domain.AdminActor("adm_1"), Reason: "traffic",
	}); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	changes, err := newLister(repo).Execute(ctx(), ports.ChangeFilter{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(changes) != 1 || changes[0].Key != domain.PricingDeliveryBase {
		t.Fatalf("changes = %+v", changes)
	}
}

func TestListChangesSurfacesAFailure(t *testing.T) {
	repo := &fakeRepo{changesErr: errors.New("boom")}
	if _, err := newLister(repo).Execute(ctx(), ports.ChangeFilter{}); errs.CodeOf(err) != "config_unavailable" {
		t.Fatalf("err = %v", err)
	}
}
