package application

import (
	"context"
	"errors"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/config/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// TunableKeys are the variables ALG-09 adjusts.
//
// Appendix B names ALG-09 "auto-tuning radius": the auto-tunable,
// distance-kind variables whose whole purpose is how far to reach for
// supply, so the same signal — merchant density and delivery failure — moves
// both in the same direction. Every other auto-tunable key (the pricing
// variables, order.cod_limit) has no signal Appendix B defines for it, so
// ALG-09 leaves them to the admin rather than inventing one.
var TunableKeys = []domain.Key{domain.DiscoveryBaseRadius, domain.DispatchPartnerRadius}

// TuneOutcome is what ALG-09 did, or did not do, with one variable.
type TuneOutcome struct {
	Key domain.Key
	// Applied is false when the pass left the value alone — comfortably
	// within range, or pinned by an admin.
	Applied bool
	// Skipped explains why, when Applied is false: "no_change_needed" or
	// "pinned". Empty when Applied is true.
	Skipped string
	// Change is the audit entry, present only when Applied is true.
	Change domain.Change
}

// AutoTuneUseCase runs ALG-09 for one area.
type AutoTuneUseCase struct {
	resolve *ResolveUseCase
	set     *SetOverrideUseCase
}

// NewAutoTuneUseCase wires the use case.
func NewAutoTuneUseCase(resolve *ResolveUseCase, set *SetOverrideUseCase) *AutoTuneUseCase {
	return &AutoTuneUseCase{resolve: resolve, set: set}
}

// Execute runs one tuning pass over every key in TunableKeys, at the area
// named by placement.AreaCode.
//
// Every write goes through SetOverrideUseCase.Execute, the same path an
// admin's own change takes — bounds, the pin check and the audit log are
// enforced once, there, not duplicated here.
func (uc *AutoTuneUseCase) Execute(ctx context.Context, placement domain.Placement, signal domain.TuningSignal) ([]TuneOutcome, error) {
	scope, err := domain.NewScope(domain.LevelArea, placement.AreaCode)
	if err != nil {
		return nil, errs.Wrap(err, errs.KindInvalid, "invalid_scope",
			"That area is not valid.")
	}

	resolved, err := uc.resolve.Execute(ctx, placement)
	if err != nil {
		return nil, err
	}
	// DiscoveryMinMerchants is a fixed, always-registered key of the right
	// kind, and Resolve populates every registered key — this cannot fail
	// for any Resolved a healthy Execute above could have returned.
	minMerchants, _ := resolved.Int(domain.DiscoveryMinMerchants)

	out := make([]TuneOutcome, 0, len(TunableKeys))
	for _, key := range TunableKeys {
		outcome, err := uc.tuneOne(ctx, key, scope, resolved, int(minMerchants), signal)
		if err != nil {
			return nil, err
		}
		out = append(out, outcome)
	}
	return out, nil
}

// tuneOne decides and, if warranted, applies ALG-09's next value for one
// key. def and current are looked up unchecked: key always comes from
// TunableKeys, a fixed list of registered keys, and resolved (built by a
// healthy Execute above) carries every registered key — neither lookup can
// fail for an input this method is ever actually called with.
func (uc *AutoTuneUseCase) tuneOne(ctx context.Context, key domain.Key, scope domain.Scope, resolved domain.Resolved, minMerchants int, signal domain.TuningSignal) (TuneOutcome, error) {
	def, _ := domain.Lookup(key)
	current, _ := resolved.Value(key)

	next, reason, changed := domain.Tune(def, current, minMerchants, signal)
	if !changed {
		return TuneOutcome{Key: key, Applied: false, Skipped: "no_change_needed"}, nil
	}

	change, err := uc.set.Execute(ctx, SetRequest{
		Key: key, Scope: scope, Value: next,
		Actor: domain.TunerActor(), Reason: reason,
	})
	if err != nil {
		if errors.Is(err, domain.ErrPinned) {
			return TuneOutcome{Key: key, Applied: false, Skipped: "pinned"}, nil
		}
		return TuneOutcome{}, err
	}
	return TuneOutcome{Key: key, Applied: true, Change: change}, nil
}
