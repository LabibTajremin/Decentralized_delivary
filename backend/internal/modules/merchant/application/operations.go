package application

import (
	"context"
	"errors"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/clock"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// OperationsUseCase is an owner running their shop day to day.
//
// Separate from registration because the rules differ: opening hours and
// holiday mode are the owner's to change at any time, approved or not, and
// never need a fresh review. Folding them into registration would put them
// behind the "frozen while under review" rule for no reason — a shop waiting on
// an admin still has to be able to say it is shut for Eid.
type OperationsUseCase struct {
	repo  ports.Repository
	clock clock.Clock
}

// NewOperationsUseCase wires the use case.
func NewOperationsUseCase(repo ports.Repository, c clock.Clock) *OperationsUseCase {
	return &OperationsUseCase{repo: repo, clock: c}
}

// HoursRequest is a week of opening times, keyed by weekday number with Sunday
// as 0, each window written "09:00-22:00". A day left out is closed.
type HoursRequest struct {
	Days map[string][]string
}

// SetHours replaces the opening schedule.
func (uc *OperationsUseCase) SetHours(ctx context.Context, ownerUserID string, req HoursRequest) (domain.Merchant, error) {
	merchant, err := uc.owned(ctx, ownerUserID)
	if err != nil {
		return domain.Merchant{}, err
	}

	hours, err := domain.DecodeWeeklyHours(req.Days)
	if err != nil {
		return domain.Merchant{}, hoursError(err)
	}
	if hours.IsAlwaysClosed() {
		// Refused rather than accepted: a schedule with no open window makes a
		// shop permanently invisible in a way that looks like a bug to its
		// owner. Closing indefinitely is what holiday mode is for, and it says
		// so on the listing.
		return domain.Merchant{}, errs.New(errs.KindInvalid, "hours_always_closed",
			"Please leave at least one day open. To close for a while, use holiday mode.")
	}

	updated := merchant.WithHours(hours)
	if err := uc.repo.Save(ctx, updated); err != nil {
		return domain.Merchant{}, unavailable(err, "We could not save your opening times just now. Please try again.")
	}
	return updated, nil
}

// HolidayRequest closes a shop for a while.
type HolidayRequest struct {
	// Until is when the shop reopens. Zero means indefinitely.
	Until  time.Time
	Reason string
}

// StartHoliday hides the shop from listings until it reopens.
func (uc *OperationsUseCase) StartHoliday(ctx context.Context, ownerUserID string, req HolidayRequest) (domain.Merchant, error) {
	merchant, err := uc.owned(ctx, ownerUserID)
	if err != nil {
		return domain.Merchant{}, err
	}

	holiday, err := domain.NewHoliday(req.Until, req.Reason, uc.clock.Now())
	if err != nil {
		return domain.Merchant{}, holidayError(err)
	}

	updated := merchant.WithHoliday(holiday)
	if err := uc.repo.Save(ctx, updated); err != nil {
		return domain.Merchant{}, unavailable(err, "We could not close your shop just now. Please try again.")
	}
	return updated, nil
}

// EndHoliday brings the shop back.
//
// Nothing is published to geo here, because holiday mode never changed the
// search index: an approved shop stays active in the index and is filtered out
// of listings by IsListed. That way a holiday costs one row write instead of a
// spatial update, and a holiday that expires on its own needs no job to notice.
func (uc *OperationsUseCase) EndHoliday(ctx context.Context, ownerUserID string) (domain.Merchant, error) {
	merchant, err := uc.owned(ctx, ownerUserID)
	if err != nil {
		return domain.Merchant{}, err
	}

	updated := merchant.WithHoliday(domain.Holiday{})
	if err := uc.repo.Save(ctx, updated); err != nil {
		return domain.Merchant{}, unavailable(err, "We could not reopen your shop just now. Please try again.")
	}
	return updated, nil
}

// owned loads the caller's shop.
func (uc *OperationsUseCase) owned(ctx context.Context, ownerUserID string) (domain.Merchant, error) {
	merchant, err := uc.repo.ByOwner(ctx, ownerUserID)
	if errors.Is(err, domain.ErrMerchantNotFound) {
		return domain.Merchant{}, errs.Wrap(err, errs.KindNotFound, "no_shop",
			"You have not registered a shop yet.")
	}
	if err != nil {
		return domain.Merchant{}, unavailable(err, "We could not load your shop just now. Please try again.")
	}
	return merchant, nil
}

// hoursError turns a schedule refusal into something an owner can act on.
func hoursError(err error) error {
	switch {
	case errors.Is(err, domain.ErrOverlappingWindows):
		return errs.Wrap(err, errs.KindInvalid, "overlapping_hours",
			"Two of those opening times overlap. Please check them.")
	case errors.Is(err, domain.ErrTooManyWindows):
		return errs.Wrap(err, errs.KindInvalid, "too_many_hours",
			"Please use at most three opening times in a day.")
	case errors.Is(err, domain.ErrInvalidWeekday):
		return errs.Wrap(err, errs.KindInvalid, "invalid_weekday", "That is not a day of the week.")
	default:
		return errs.Wrap(err, errs.KindInvalid, "invalid_hours",
			"Please write opening times like 09:00-22:00.")
	}
}

// holidayError turns a holiday refusal into something an owner can act on.
func holidayError(err error) error {
	switch {
	case errors.Is(err, domain.ErrHolidayInThePast):
		return errs.Wrap(err, errs.KindInvalid, "holiday_in_the_past",
			"Please choose a date in the future.")
	case errors.Is(err, domain.ErrHolidayTooLong):
		return errs.Wrap(err, errs.KindInvalid, "holiday_too_long",
			"Please choose a date within the next two months.")
	default:
		return errs.Wrap(err, errs.KindInvalid, "invalid_holiday", "Those holiday details are not valid.")
	}
}
