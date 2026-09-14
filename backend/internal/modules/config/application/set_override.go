package application

import (
	"context"
	"errors"
	"strings"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/config/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/config/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/clock"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/id"
)

// SetOverrideUseCase applies an admin or auto-tuner change.
//
// Every write goes through here. There is no second path that skips the checks,
// which is what makes the guarantees below true rather than merely intended.
type SetOverrideUseCase struct {
	repo  ports.ConfigRepository
	clock clock.Clock
	ids   id.Generator
}

// NewSetOverrideUseCase wires the use case.
func NewSetOverrideUseCase(repo ports.ConfigRepository, c clock.Clock, ids id.Generator) *SetOverrideUseCase {
	return &SetOverrideUseCase{repo: repo, clock: c, ids: ids}
}

// SetRequest is one change.
type SetRequest struct {
	Key    domain.Key
	Scope  domain.Scope
	Value  domain.Value
	Pinned bool
	Actor  domain.Actor
	Reason string
}

// Execute validates and applies a change, recording it in the audit log.
//
// The order of checks matters and is deliberate: identity of the key, then who
// is asking, then whether the value is allowed. A tuner attempting to change an
// immutable variable is told it is immutable, not that it is not auto-tunable —
// the first is the real reason and the second would send someone looking in the
// wrong place.
func (uc *SetOverrideUseCase) Execute(ctx context.Context, req SetRequest) (domain.Change, error) {
	def, err := domain.Lookup(req.Key)
	if err != nil {
		return domain.Change{}, errs.Wrap(err, errs.KindNotFound, "unknown_config_key",
			"That setting does not exist.").With("key", string(req.Key))
	}

	// D3: the division ceiling is not adjustable by anyone. This is checked
	// before the actor is considered, because the answer is the same for an
	// admin, the tuner, and anything else that might be added later.
	if def.Immutable {
		return domain.Change{}, errs.Wrap(domain.ErrImmutable, errs.KindForbidden, "immutable_config_key",
			"This setting protects a rule that cannot be turned off.").
			With("key", string(req.Key))
	}

	if req.Actor.Kind == domain.ActorAutoTuner {
		if err := uc.checkTunerMayWrite(ctx, def, req); err != nil {
			return domain.Change{}, err
		}
	}

	if err := def.InBounds(req.Value); err != nil {
		return domain.Change{}, uc.boundsError(def, err)
	}

	if strings.TrimSpace(req.Reason) == "" {
		return domain.Change{}, errs.New(errs.KindInvalid, "reason_required",
			"Please say why this setting is being changed.")
	}

	previous, err := uc.currentValue(ctx, def, req.Scope)
	if err != nil {
		return domain.Change{}, err
	}

	change := domain.Change{
		ID:       uc.ids.New("cfg"),
		Key:      req.Key,
		Scope:    req.Scope,
		OldValue: previous,
		NewValue: req.Value,
		Actor:    req.Actor,
		Reason:   strings.TrimSpace(req.Reason),
		At:       uc.clock.Now(),
	}

	override := domain.Override{
		Key:    req.Key,
		Scope:  req.Scope,
		Value:  req.Value,
		Pinned: req.Pinned,
	}
	if err := uc.repo.SaveOverride(ctx, override, change); err != nil {
		return domain.Change{}, errs.Wrap(err, errs.KindUnavailable, "config_write_failed",
			"We could not save that setting. Please try again.")
	}
	return change, nil
}

// checkTunerMayWrite applies the two limits on ALG-09.
func (uc *SetOverrideUseCase) checkTunerMayWrite(ctx context.Context, def domain.Definition, req SetRequest) error {
	if !def.AutoTunable {
		return errs.Wrap(domain.ErrNotAutoTunable, errs.KindForbidden, "not_auto_tunable",
			"This setting is only changed by an administrator.").
			With("key", string(req.Key))
	}

	// A pin is an admin saying "I have decided this one". The tuner must not
	// undo that, so the pin is checked at the exact scope being written: a pin
	// in one area does not freeze another.
	existing, err := uc.repo.AllOverrides(ctx, req.Scope)
	if err != nil {
		return errs.Wrap(err, errs.KindUnavailable, "config_unavailable",
			"We could not check this setting. Please try again.")
	}
	for _, o := range existing {
		if o.Key == req.Key && o.Pinned {
			return errs.Wrap(domain.ErrPinned, errs.KindConflict, "config_pinned",
				"An administrator has pinned this setting.").
				With("key", string(req.Key)).
				With("scope", req.Scope.String())
		}
	}
	return nil
}

// boundsError turns a domain bounds failure into a message an admin can act on.
func (uc *SetOverrideUseCase) boundsError(def domain.Definition, err error) error {
	if errors.Is(err, domain.ErrKindMismatch) {
		return errs.Wrap(err, errs.KindInvalid, "wrong_config_type",
			"That value is the wrong type for this setting.").
			With("key", string(def.Key)).
			With("expected", def.Kind.String())
	}
	return errs.Wrap(err, errs.KindInvalid, "config_out_of_bounds",
		"That value is outside the allowed range for this setting.").
		With("key", string(def.Key)).
		With("minimum", def.Min.String()).
		With("maximum", def.Max.String())
}

// currentValue finds what the value is now at this exact scope, for the audit
// entry. Absent means the scope had no override, so the recorded old value is
// the definition's default — which is what the change is actually moving away
// from at that scope.
func (uc *SetOverrideUseCase) currentValue(ctx context.Context, def domain.Definition, scope domain.Scope) (domain.Value, error) {
	existing, err := uc.repo.AllOverrides(ctx, scope)
	if err != nil {
		return domain.Value{}, errs.Wrap(err, errs.KindUnavailable, "config_unavailable",
			"We could not read the current setting. Please try again.")
	}
	for _, o := range existing {
		if o.Key == def.Key {
			return o.Value, nil
		}
	}
	return def.Default, nil
}

// ClearOverrideUseCase removes an override so the next scope up applies again.
type ClearOverrideUseCase struct {
	repo  ports.ConfigRepository
	clock clock.Clock
	ids   id.Generator
}

// NewClearOverrideUseCase wires the use case.
func NewClearOverrideUseCase(repo ports.ConfigRepository, c clock.Clock, ids id.Generator) *ClearOverrideUseCase {
	return &ClearOverrideUseCase{repo: repo, clock: c, ids: ids}
}

// Execute removes an override, recording the removal.
//
// Clearing is a change like any other and is audited the same way. "Why did
// this area go back to the default?" is the same question as "why did it
// change", and an unrecorded removal makes it unanswerable.
func (uc *ClearOverrideUseCase) Execute(ctx context.Context, key domain.Key, scope domain.Scope, actor domain.Actor, reason string) (domain.Change, error) {
	def, err := domain.Lookup(key)
	if err != nil {
		return domain.Change{}, errs.Wrap(err, errs.KindNotFound, "unknown_config_key",
			"That setting does not exist.").With("key", string(key))
	}
	if scope.Level == domain.LevelGlobal {
		return domain.Change{}, errs.New(errs.KindInvalid, "cannot_clear_global",
			"The global value is the fallback and cannot be removed.").
			With("key", string(key))
	}
	if strings.TrimSpace(reason) == "" {
		return domain.Change{}, errs.New(errs.KindInvalid, "reason_required",
			"Please say why this setting is being reset.")
	}

	existing, err := uc.repo.AllOverrides(ctx, scope)
	if err != nil {
		return domain.Change{}, errs.Wrap(err, errs.KindUnavailable, "config_unavailable",
			"We could not read the current setting. Please try again.")
	}
	var old domain.Value
	found := false
	for _, o := range existing {
		if o.Key == key {
			old, found = o.Value, true
			break
		}
	}
	if !found {
		return domain.Change{}, errs.New(errs.KindNotFound, "no_override_here",
			"There is no setting to reset at this level.").
			With("key", string(key)).
			With("scope", scope.String())
	}

	change := domain.Change{
		ID:       uc.ids.New("cfg"),
		Key:      key,
		Scope:    scope,
		OldValue: old,
		NewValue: def.Default,
		Actor:    actor,
		Reason:   strings.TrimSpace(reason),
		At:       uc.clock.Now(),
	}
	if err := uc.repo.DeleteOverride(ctx, key, scope, change); err != nil {
		return domain.Change{}, errs.Wrap(err, errs.KindUnavailable, "config_write_failed",
			"We could not reset that setting. Please try again.")
	}
	return change, nil
}
