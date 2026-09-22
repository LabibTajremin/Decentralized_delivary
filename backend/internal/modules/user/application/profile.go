// Package application holds the user use cases.
package application

import (
	"context"
	"errors"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/user/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/user/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// ProfileUseCase reads and updates profiles.
type ProfileUseCase struct {
	profiles ports.ProfileRepository
}

// NewProfileUseCase wires the use case.
func NewProfileUseCase(profiles ports.ProfileRepository) *ProfileUseCase {
	return &ProfileUseCase{profiles: profiles}
}

// Get returns a profile, inventing an empty one for a user who has never set
// anything.
//
// A missing profile is not an error. Identity creates an account on first
// sign-in and the customer may never open the profile screen; returning 404
// would make every client handle "signed in but has no profile" as a special
// case, and some of them would handle it wrong.
func (uc *ProfileUseCase) Get(ctx context.Context, userID string) (domain.Profile, error) {
	profile, err := uc.profiles.Profile(ctx, userID)
	if err == nil {
		return profile, nil
	}
	if errors.Is(err, domain.ErrProfileNotFound) {
		empty, buildErr := domain.NewProfile(userID, "", "", domain.DefaultLanguage)
		if buildErr != nil {
			// Only reachable with an empty user id, which the transport guard
			// prevents. Wrapped anyway: an unwrapped domain error would reach a
			// client as an opaque 500 with no code to act on.
			return domain.Profile{}, profileError(buildErr)
		}
		return empty, nil
	}
	return domain.Profile{}, errs.Wrap(err, errs.KindUnavailable, "profile_unavailable",
		"We could not load your profile just now. Please try again.")
}

// UpdateRequest is a profile change.
//
// Every field is a pointer so "leave this alone" and "clear this" are different
// requests. Without that, a client updating only the name would have to send
// the email back too, and one that forgot would silently erase it.
type UpdateRequest struct {
	Name     *string
	Email    *string
	Language *string
}

// Update applies a change and returns the stored profile.
func (uc *ProfileUseCase) Update(ctx context.Context, userID string, req UpdateRequest) (domain.Profile, error) {
	current, err := uc.Get(ctx, userID)
	if err != nil {
		return domain.Profile{}, err
	}

	name, email, language := current.Name, current.Email, current.Language
	if req.Name != nil {
		name = *req.Name
	}
	if req.Email != nil {
		email = *req.Email
	}
	if req.Language != nil {
		parsed, parseErr := domain.ParseLanguage(*req.Language)
		if parseErr != nil {
			return domain.Profile{}, errs.Wrap(parseErr, errs.KindInvalid, "unsupported_language",
				"We do not support that language yet.")
		}
		language = parsed
	}

	updated, err := domain.NewProfile(userID, name, email, language)
	if err != nil {
		return domain.Profile{}, profileError(err)
	}
	if err := uc.profiles.SaveProfile(ctx, updated); err != nil {
		return domain.Profile{}, errs.Wrap(err, errs.KindUnavailable, "profile_write_failed",
			"We could not save your profile just now. Please try again.")
	}
	return updated, nil
}

// profileError turns a domain refusal into something a person can act on.
//
// Language failures are not listed: Update parses the language before building
// the profile, so NewProfile never sees an unsupported one from this path.
// Listing it would be a branch no test could reach.
func profileError(err error) error {
	switch {
	case errors.Is(err, domain.ErrNameTooLong):
		return errs.Wrap(err, errs.KindInvalid, "name_too_long",
			"That name is too long.")
	case errors.Is(err, domain.ErrInvalidEmail):
		return errs.Wrap(err, errs.KindInvalid, "invalid_email",
			"That email address does not look valid.")
	default:
		return errs.Wrap(err, errs.KindInvalid, "invalid_profile",
			"Some of those details are not valid.")
	}
}
