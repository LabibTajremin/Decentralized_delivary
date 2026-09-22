package config

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/config/application"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/config/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/config/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// fakeRepo is an in-memory ConfigRepository. It keeps overrides and changes
// together so a test can assert that a write really did both — a change that is
// applied but not recorded is exactly the change someone needs to explain later.
type fakeRepo struct {
	overrides []domain.Override
	changes   []domain.Change

	loadErr    error
	writeErr   error
	scopeErr   error
	changesErr error
}

func (f *fakeRepo) OverridesFor(context.Context, domain.Placement) ([]domain.Override, error) {
	if f.loadErr != nil {
		return nil, f.loadErr
	}
	return f.overrides, nil
}

func (f *fakeRepo) AllOverrides(_ context.Context, scope domain.Scope) ([]domain.Override, error) {
	if f.scopeErr != nil {
		return nil, f.scopeErr
	}
	var out []domain.Override
	for _, o := range f.overrides {
		if o.Scope == scope {
			out = append(out, o)
		}
	}
	return out, nil
}

func (f *fakeRepo) SaveOverride(_ context.Context, o domain.Override, c domain.Change) error {
	if f.writeErr != nil {
		return f.writeErr
	}
	for i, existing := range f.overrides {
		if existing.Key == o.Key && existing.Scope == o.Scope {
			f.overrides[i] = o
			f.changes = append(f.changes, c)
			return nil
		}
	}
	f.overrides = append(f.overrides, o)
	f.changes = append(f.changes, c)
	return nil
}

func (f *fakeRepo) DeleteOverride(_ context.Context, key domain.Key, scope domain.Scope, c domain.Change) error {
	if f.writeErr != nil {
		return f.writeErr
	}
	kept := f.overrides[:0]
	for _, o := range f.overrides {
		if o.Key == key && o.Scope == scope {
			continue
		}
		kept = append(kept, o)
	}
	f.overrides = kept
	f.changes = append(f.changes, c)
	return nil
}

func (f *fakeRepo) Changes(_ context.Context, filter ports.ChangeFilter) ([]domain.Change, error) {
	if f.changesErr != nil {
		return nil, f.changesErr
	}
	out := make([]domain.Change, 0, len(f.changes))
	for _, c := range f.changes {
		if filter.Key != "" && c.Key != filter.Key {
			continue
		}
		if filter.Scope != nil && c.Scope != *filter.Scope {
			continue
		}
		out = append(out, c)
	}
	if filter.Limit > 0 && len(out) > filter.Limit {
		out = out[:filter.Limit]
	}
	return out, nil
}

// fixedClock and fixedIDs make audit entries assertable.
type fixedClock struct{ at time.Time }

func (f fixedClock) Now() time.Time { return f.at }

type seqIDs struct{ n int }

func (s *seqIDs) New(prefix string) string {
	s.n++
	return prefix + "_" + string(rune('a'+s.n-1))
}

var changedAt = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

func newSetter(repo *fakeRepo) *application.SetOverrideUseCase {
	return application.NewSetOverrideUseCase(repo, fixedClock{at: changedAt}, &seqIDs{})
}

func newClearer(repo *fakeRepo) *application.ClearOverrideUseCase {
	return application.NewClearOverrideUseCase(repo, fixedClock{at: changedAt}, &seqIDs{})
}

func areaScope(t *testing.T) domain.Scope {
	t.Helper()
	s, err := domain.NewScope(domain.LevelArea, "DHK-DHM")
	if err != nil {
		t.Fatalf("NewScope: %v", err)
	}
	return s
}

func ctx() context.Context { return context.Background() }

// TestAnAdminCanRaiseAFeeWithinBounds is the ordinary case.
func TestAnAdminCanRaiseAFeeWithinBounds(t *testing.T) {
	repo := &fakeRepo{}
	value, _ := domain.Money(6000)

	change, err := newSetter(repo).Execute(ctx(), application.SetRequest{
		Key:    domain.PricingDeliveryBase,
		Scope:  areaScope(t),
		Value:  value,
		Actor:  domain.AdminActor("adm_1"),
		Reason: "Dhanmondi traffic has got worse",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(repo.overrides) != 1 || !repo.overrides[0].Value.Equal(value) {
		t.Errorf("overrides = %+v", repo.overrides)
	}
	if change.OldValue.String() != "4000" {
		t.Errorf("old value = %s, want the previous effective value at this scope", change.OldValue)
	}
	if change.NewValue.String() != "6000" || change.At != changedAt {
		t.Errorf("change = %+v", change)
	}
	if change.Actor.ID != "adm_1" || change.Reason != "Dhanmondi traffic has got worse" {
		t.Errorf("change = %+v", change)
	}
}

// TestEveryChangeIsAudited is D5. A change applied but not recorded makes
// "why is this fee different" unanswerable.
func TestEveryChangeIsAudited(t *testing.T) {
	repo := &fakeRepo{}
	value, _ := domain.Money(6000)
	if _, err := newSetter(repo).Execute(ctx(), application.SetRequest{
		Key:    domain.PricingDeliveryBase,
		Scope:  areaScope(t),
		Value:  value,
		Actor:  domain.AdminActor("adm_1"),
		Reason: "trial",
	}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(repo.changes) != 1 {
		t.Fatalf("audit entries = %d, want 1", len(repo.changes))
	}
	if repo.changes[0].Key != domain.PricingDeliveryBase {
		t.Errorf("audit entry = %+v", repo.changes[0])
	}
}

// TestAChangeWithoutAReasonIsRefused: an audit log full of unexplained changes
// answers "what" but never "why", which is the question that actually gets
// asked.
func TestAChangeWithoutAReasonIsRefused(t *testing.T) {
	repo := &fakeRepo{}
	value, _ := domain.Money(6000)
	for _, reason := range []string{"", "   "} {
		_, err := newSetter(repo).Execute(ctx(), application.SetRequest{
			Key:    domain.PricingDeliveryBase,
			Scope:  areaScope(t),
			Value:  value,
			Actor:  domain.AdminActor("adm_1"),
			Reason: reason,
		})
		if errs.CodeOf(err) != "reason_required" {
			t.Errorf("reason %q: error = %v, want reason_required", reason, err)
		}
	}
	if len(repo.changes) != 0 {
		t.Error("a refused change must write nothing")
	}
}

// TestNobodyCanDisableTheDivisionCeiling is the system's one hard invariant.
func TestNobodyCanDisableTheDivisionCeiling(t *testing.T) {
	for _, actor := range []domain.Actor{domain.AdminActor("adm_1"), domain.TunerActor()} {
		repo := &fakeRepo{}
		_, err := newSetter(repo).Execute(ctx(), application.SetRequest{
			Key:    domain.DiscoveryDivisionCeil,
			Scope:  areaScope(t),
			Value:  domain.Bool(false),
			Actor:  actor,
			Reason: "we want to serve across divisions",
		})
		if !errors.Is(err, domain.ErrImmutable) {
			t.Errorf("%s: error = %v, want ErrImmutable", actor.Kind, err)
		}
		if errs.KindOf(err) != errs.KindForbidden {
			t.Errorf("%s: kind = %v, want forbidden", actor.Kind, errs.KindOf(err))
		}
		if len(repo.overrides) != 0 || len(repo.changes) != 0 {
			t.Errorf("%s: a refused change wrote something", actor.Kind)
		}
	}
}

// Even setting it to its own value is refused: there is no legitimate write to
// this key, and allowing a no-op write invites someone to find the code path.
func TestTheDivisionCeilingCannotEvenBeSetToTrue(t *testing.T) {
	repo := &fakeRepo{}
	_, err := newSetter(repo).Execute(ctx(), application.SetRequest{
		Key:    domain.DiscoveryDivisionCeil,
		Scope:  domain.GlobalScope,
		Value:  domain.Bool(true),
		Actor:  domain.AdminActor("adm_1"),
		Reason: "belt and braces",
	})
	if !errors.Is(err, domain.ErrImmutable) {
		t.Errorf("error = %v, want ErrImmutable", err)
	}
}

// TestTheTunerCannotTouchNonTunableVariables is the first ALG-09 limit.
func TestTheTunerCannotTouchNonTunableVariables(t *testing.T) {
	repo := &fakeRepo{}
	value, _ := domain.Count(9)
	_, err := newSetter(repo).Execute(ctx(), application.SetRequest{
		Key:    domain.DiscoveryMaxExpansions,
		Scope:  areaScope(t),
		Value:  value,
		Actor:  domain.TunerActor(),
		Reason: "more expansions would find more merchants",
	})
	if !errors.Is(err, domain.ErrNotAutoTunable) {
		t.Errorf("error = %v, want ErrNotAutoTunable", err)
	}
	if errs.KindOf(err) != errs.KindForbidden {
		t.Errorf("kind = %v, want forbidden", errs.KindOf(err))
	}
}

// An admin may change a variable the tuner cannot. The restriction is on the
// tuner, not on the variable.
func TestAnAdminCanChangeANonTunableVariable(t *testing.T) {
	repo := &fakeRepo{}
	value, _ := domain.Count(9)
	if _, err := newSetter(repo).Execute(ctx(), application.SetRequest{
		Key:    domain.DiscoveryMaxExpansions,
		Scope:  areaScope(t),
		Value:  value,
		Actor:  domain.AdminActor("adm_1"),
		Reason: "rural areas need more steps",
	}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(repo.overrides) != 1 {
		t.Errorf("overrides = %+v", repo.overrides)
	}
}

// TestTheTunerCannotExceedItsBounds is the second ALG-09 limit, and the one
// that makes the tuner safe to run unattended.
func TestTheTunerCannotExceedItsBounds(t *testing.T) {
	repo := &fakeRepo{}
	absurd, _ := domain.Money(1_000_000) // ৳10,000 delivery fee
	_, err := newSetter(repo).Execute(ctx(), application.SetRequest{
		Key:    domain.PricingDeliveryBase,
		Scope:  areaScope(t),
		Value:  absurd,
		Actor:  domain.TunerActor(),
		Reason: "demand is very high",
	})
	if !errors.Is(err, domain.ErrOutOfBounds) {
		t.Fatalf("error = %v, want ErrOutOfBounds", err)
	}
	if errs.CodeOf(err) != "config_out_of_bounds" {
		t.Errorf("code = %q", errs.CodeOf(err))
	}
	var e *errs.Error
	if errors.As(err, &e) {
		if e.Fields()["maximum"] == "" {
			t.Error("the error must tell the caller what the maximum is")
		}
	}
}

// An admin is bounded too. Bounds are a property of the variable, not a
// restraint on one actor.
func TestAnAdminIsAlsoBounded(t *testing.T) {
	repo := &fakeRepo{}
	absurd, _ := domain.Money(1_000_000)
	_, err := newSetter(repo).Execute(ctx(), application.SetRequest{
		Key:    domain.PricingDeliveryBase,
		Scope:  areaScope(t),
		Value:  absurd,
		Actor:  domain.AdminActor("adm_1"),
		Reason: "special event",
	})
	if !errors.Is(err, domain.ErrOutOfBounds) {
		t.Errorf("error = %v, want ErrOutOfBounds", err)
	}
}

// TestAPinnedVariableResistsTheTuner: a pin is an admin saying "I have decided
// this one", and the tuner must not undo that.
func TestAPinnedVariableResistsTheTuner(t *testing.T) {
	scope := areaScope(t)
	pinned, _ := domain.Money(7000)
	repo := &fakeRepo{overrides: []domain.Override{
		{Key: domain.PricingDeliveryBase, Scope: scope, Value: pinned, Pinned: true},
	}}

	newValue, _ := domain.Money(5000)
	_, err := newSetter(repo).Execute(ctx(), application.SetRequest{
		Key:    domain.PricingDeliveryBase,
		Scope:  scope,
		Value:  newValue,
		Actor:  domain.TunerActor(),
		Reason: "demand has fallen",
	})
	if !errors.Is(err, domain.ErrPinned) {
		t.Fatalf("error = %v, want ErrPinned", err)
	}
	if errs.KindOf(err) != errs.KindConflict {
		t.Errorf("kind = %v, want conflict", errs.KindOf(err))
	}
	if !repo.overrides[0].Value.Equal(pinned) {
		t.Error("the pinned value was changed")
	}
}

// An admin can always override their own pin.
func TestAnAdminCanChangeAPinnedVariable(t *testing.T) {
	scope := areaScope(t)
	pinned, _ := domain.Money(7000)
	repo := &fakeRepo{overrides: []domain.Override{
		{Key: domain.PricingDeliveryBase, Scope: scope, Value: pinned, Pinned: true},
	}}

	newValue, _ := domain.Money(5000)
	if _, err := newSetter(repo).Execute(ctx(), application.SetRequest{
		Key:    domain.PricingDeliveryBase,
		Scope:  scope,
		Value:  newValue,
		Pinned: true,
		Actor:  domain.AdminActor("adm_1"),
		Reason: "reconsidered",
	}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !repo.overrides[0].Value.Equal(newValue) {
		t.Errorf("value = %s, want the admin's change applied", repo.overrides[0].Value)
	}
}

// A pin in one area must not freeze another.
func TestAPinInOneAreaDoesNotFreezeAnother(t *testing.T) {
	pinnedScope, _ := domain.NewScope(domain.LevelArea, "DHK-GUL")
	pinned, _ := domain.Money(7000)
	repo := &fakeRepo{overrides: []domain.Override{
		{Key: domain.PricingDeliveryBase, Scope: pinnedScope, Value: pinned, Pinned: true},
	}}

	newValue, _ := domain.Money(5000)
	if _, err := newSetter(repo).Execute(ctx(), application.SetRequest{
		Key:    domain.PricingDeliveryBase,
		Scope:  areaScope(t), // a different area
		Value:  newValue,
		Actor:  domain.TunerActor(),
		Reason: "demand has fallen in Dhanmondi",
	}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
}

func TestSettingAnUnknownKeyIsRefused(t *testing.T) {
	repo := &fakeRepo{}
	value, _ := domain.Money(1000)
	_, err := newSetter(repo).Execute(ctx(), application.SetRequest{
		Key:    "pricing.delivery_bass",
		Scope:  areaScope(t),
		Value:  value,
		Actor:  domain.AdminActor("adm_1"),
		Reason: "typo",
	})
	if errs.CodeOf(err) != "unknown_config_key" || errs.KindOf(err) != errs.KindNotFound {
		t.Errorf("error = %v (%v)", err, errs.KindOf(err))
	}
}

func TestSettingTheWrongTypeIsRefused(t *testing.T) {
	repo := &fakeRepo{}
	wrong, _ := domain.Distance(5000)
	_, err := newSetter(repo).Execute(ctx(), application.SetRequest{
		Key:    domain.PricingDeliveryBase,
		Scope:  areaScope(t),
		Value:  wrong,
		Actor:  domain.AdminActor("adm_1"),
		Reason: "wrong units",
	})
	if errs.CodeOf(err) != "wrong_config_type" {
		t.Errorf("error = %v, want wrong_config_type", err)
	}
}

func TestSetSurfacesAWriteFailure(t *testing.T) {
	repo := &fakeRepo{writeErr: errors.New("disk full")}
	value, _ := domain.Money(6000)
	_, err := newSetter(repo).Execute(ctx(), application.SetRequest{
		Key:    domain.PricingDeliveryBase,
		Scope:  areaScope(t),
		Value:  value,
		Actor:  domain.AdminActor("adm_1"),
		Reason: "trial",
	})
	if errs.KindOf(err) != errs.KindUnavailable {
		t.Errorf("error = %v, want unavailable", err)
	}
}

func TestSetSurfacesAScopeReadFailure(t *testing.T) {
	repo := &fakeRepo{scopeErr: errors.New("connection reset")}
	value, _ := domain.Money(6000)
	_, err := newSetter(repo).Execute(ctx(), application.SetRequest{
		Key:    domain.PricingDeliveryBase,
		Scope:  areaScope(t),
		Value:  value,
		Actor:  domain.AdminActor("adm_1"),
		Reason: "trial",
	})
	if errs.KindOf(err) != errs.KindUnavailable {
		t.Errorf("error = %v, want unavailable", err)
	}
}

func TestTunerPinCheckSurfacesAReadFailure(t *testing.T) {
	repo := &fakeRepo{scopeErr: errors.New("connection reset")}
	value, _ := domain.Money(5000)
	_, err := newSetter(repo).Execute(ctx(), application.SetRequest{
		Key:    domain.PricingDeliveryBase,
		Scope:  areaScope(t),
		Value:  value,
		Actor:  domain.TunerActor(),
		Reason: "tuning",
	})
	if errs.KindOf(err) != errs.KindUnavailable {
		t.Errorf("error = %v, want unavailable", err)
	}
}

// TestClearingRevertsToTheNextScopeUp and records why.
func TestClearingRevertsToTheNextScopeUp(t *testing.T) {
	scope := areaScope(t)
	areaValue, _ := domain.Money(6000)
	repo := &fakeRepo{overrides: []domain.Override{
		{Key: domain.PricingDeliveryBase, Scope: scope, Value: areaValue},
	}}

	change, err := newClearer(repo).Execute(ctx(), domain.PricingDeliveryBase, scope,
		domain.AdminActor("adm_1"), "the trial is over")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(repo.overrides) != 0 {
		t.Errorf("overrides = %+v, want the override removed", repo.overrides)
	}
	if len(repo.changes) != 1 {
		t.Fatalf("audit entries = %d, want the removal recorded", len(repo.changes))
	}
	if change.OldValue.String() != "6000" || change.NewValue.String() != "4000" {
		t.Errorf("change = %s -> %s, want 6000 -> the default 4000", change.OldValue, change.NewValue)
	}
}

// The global value is the fallback that makes resolution total. Removing it
// would leave keys with no value at all.
func TestTheGlobalValueCannotBeCleared(t *testing.T) {
	repo := &fakeRepo{}
	_, err := newClearer(repo).Execute(ctx(), domain.PricingDeliveryBase, domain.GlobalScope,
		domain.AdminActor("adm_1"), "tidying up")
	if errs.CodeOf(err) != "cannot_clear_global" {
		t.Errorf("error = %v, want cannot_clear_global", err)
	}
}

func TestClearingSomethingThatIsNotThereIsRefused(t *testing.T) {
	repo := &fakeRepo{}
	_, err := newClearer(repo).Execute(ctx(), domain.PricingDeliveryBase, areaScope(t),
		domain.AdminActor("adm_1"), "tidying up")
	if errs.CodeOf(err) != "no_override_here" || errs.KindOf(err) != errs.KindNotFound {
		t.Errorf("error = %v", err)
	}
}

func TestClearingRequiresAReason(t *testing.T) {
	scope := areaScope(t)
	value, _ := domain.Money(6000)
	repo := &fakeRepo{overrides: []domain.Override{
		{Key: domain.PricingDeliveryBase, Scope: scope, Value: value},
	}}
	_, err := newClearer(repo).Execute(ctx(), domain.PricingDeliveryBase, scope,
		domain.AdminActor("adm_1"), "  ")
	if errs.CodeOf(err) != "reason_required" {
		t.Errorf("error = %v, want reason_required", err)
	}
}

func TestClearingAnUnknownKeyIsRefused(t *testing.T) {
	repo := &fakeRepo{}
	_, err := newClearer(repo).Execute(ctx(), "nope", areaScope(t),
		domain.AdminActor("adm_1"), "tidying")
	if errs.CodeOf(err) != "unknown_config_key" {
		t.Errorf("error = %v", err)
	}
}

func TestClearSurfacesReadAndWriteFailures(t *testing.T) {
	scope := areaScope(t)
	value, _ := domain.Money(6000)

	readFail := &fakeRepo{scopeErr: errors.New("connection reset")}
	if _, err := newClearer(readFail).Execute(ctx(), domain.PricingDeliveryBase, scope,
		domain.AdminActor("adm_1"), "reason"); errs.KindOf(err) != errs.KindUnavailable {
		t.Errorf("read failure = %v", err)
	}

	writeFail := &fakeRepo{
		overrides: []domain.Override{{Key: domain.PricingDeliveryBase, Scope: scope, Value: value}},
		writeErr:  errors.New("disk full"),
	}
	if _, err := newClearer(writeFail).Execute(ctx(), domain.PricingDeliveryBase, scope,
		domain.AdminActor("adm_1"), "reason"); errs.KindOf(err) != errs.KindUnavailable {
		t.Errorf("write failure = %v", err)
	}
}

// TestResolveFailsRatherThanSilentlyServingDefaults: quietly falling back
// during a database outage would silently shrink a 15 km area to 5 km and tell
// customers there are no merchants near them.
func TestResolveFailsRatherThanSilentlyServingDefaults(t *testing.T) {
	repo := &fakeRepo{loadErr: errors.New("connection refused")}
	_, err := application.NewResolveUseCase(repo).Execute(ctx(), dhanmondi)
	if errs.KindOf(err) != errs.KindUnavailable {
		t.Fatalf("error = %v, want unavailable", err)
	}
	if errs.CodeOf(err) != "config_unavailable" {
		t.Errorf("code = %q", errs.CodeOf(err))
	}
}

func TestResolveAppliesStoredOverrides(t *testing.T) {
	value, _ := domain.Money(6000)
	repo := &fakeRepo{overrides: []domain.Override{
		{Key: domain.PricingDeliveryBase, Scope: areaScope(t), Value: value},
	}}
	resolved, err := application.NewResolveUseCase(repo).Execute(ctx(), dhanmondi)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got, err := resolved.Int(domain.PricingDeliveryBase)
	if err != nil || got != 6000 {
		t.Errorf("delivery base = %d, %v", got, err)
	}
}
