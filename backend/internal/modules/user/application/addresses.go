package application

import (
	"context"
	"errors"

	geoext "github.com/rootlogic-lab/delivery/backend/internal/modules/user/external/geo"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/user/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/user/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/id"
)

// maxAddressesPerUser caps the address book.
//
// Twenty is far more than anyone needs and low enough that the list stays
// readable and the table stays small. Refusing rather than silently dropping
// the oldest: an address is something the customer typed, and quietly losing
// one is worse than telling them the book is full.
const maxAddressesPerUser = 20

// AddressUseCase manages a user's address book.
type AddressUseCase struct {
	addresses ports.AddressRepository
	geo       geoext.Service
	ids       id.Generator
}

// NewAddressUseCase wires the use case.
func NewAddressUseCase(addresses ports.AddressRepository, geo geoext.Service, ids id.Generator) *AddressUseCase {
	return &AddressUseCase{addresses: addresses, geo: geo, ids: ids}
}

// List returns a user's addresses.
func (uc *AddressUseCase) List(ctx context.Context, userID string) ([]domain.Address, error) {
	addresses, err := uc.addresses.Addresses(ctx, userID)
	if err != nil {
		return nil, errs.Wrap(err, errs.KindUnavailable, "addresses_unavailable",
			"We could not load your addresses just now. Please try again.")
	}
	return addresses, nil
}

// AddressRequest is a new or edited address.
type AddressRequest struct {
	Label          string
	RecipientName  string
	RecipientPhone string
	Line1          string
	Line2          string
	Instructions   string
	Lat            float64
	Lng            float64
	MakeDefault    bool
}

// Add places a new address and stores it.
//
// The address is resolved to an area before it is saved, and an address that
// cannot be placed is refused. This is the acceptance criterion for the phase,
// and the reason is downstream: config resolution, pricing and dispatch all key
// off the area, so an unplaced address is an order that gets as far as checkout
// and then cannot be priced.
func (uc *AddressUseCase) Add(ctx context.Context, userID string, req AddressRequest) (domain.Address, error) {
	existing, err := uc.addresses.Addresses(ctx, userID)
	if err != nil {
		return domain.Address{}, errs.Wrap(err, errs.KindUnavailable, "addresses_unavailable",
			"We could not save that address just now. Please try again.")
	}
	if len(existing) >= maxAddressesPerUser {
		return domain.Address{}, errs.New(errs.KindConflict, "address_book_full",
			"You have saved as many addresses as we can keep. Please remove one first.")
	}

	address, err := uc.build(uc.ids.New("adr"), userID, req)
	if err != nil {
		return domain.Address{}, err
	}
	placed, err := uc.place(ctx, address)
	if err != nil {
		return domain.Address{}, err
	}

	// The first address a user saves is their default whether they asked or
	// not: a customer with one address and no default would be asked to choose
	// between one option at checkout.
	placed.IsDefault = req.MakeDefault || len(existing) == 0

	updated := append(existing, placed)
	if placed.IsDefault {
		updated = promote(updated, placed.ID)
	}
	if err := uc.save(ctx, userID, updated); err != nil {
		return domain.Address{}, err
	}
	return placed, nil
}

// Update edits an existing address, re-placing it.
//
// Re-placed on every edit, not only when the pin moves. A customer who corrects
// their street name has usually also nudged the pin, and an address whose
// stored area no longer matches its location prices the wrong zone — silently,
// and in the customer's favour or ours at random.
func (uc *AddressUseCase) Update(ctx context.Context, userID, addressID string, req AddressRequest) (domain.Address, error) {
	existing, err := uc.addresses.Addresses(ctx, userID)
	if err != nil {
		return domain.Address{}, errs.Wrap(err, errs.KindUnavailable, "addresses_unavailable",
			"We could not save that address just now. Please try again.")
	}

	index := -1
	for i := range existing {
		if existing[i].ID == addressID {
			index = i
			break
		}
	}
	if index == -1 {
		return domain.Address{}, notFound(addressID)
	}

	address, err := uc.build(addressID, userID, req)
	if err != nil {
		return domain.Address{}, err
	}
	placed, err := uc.place(ctx, address)
	if err != nil {
		return domain.Address{}, err
	}
	placed.IsDefault = existing[index].IsDefault || req.MakeDefault

	updated := make([]domain.Address, len(existing))
	copy(updated, existing)
	updated[index] = placed
	if placed.IsDefault {
		updated = promote(updated, addressID)
	}
	if err := uc.save(ctx, userID, updated); err != nil {
		return domain.Address{}, err
	}
	return placed, nil
}

// Delete removes an address, promoting another default if it was the default.
func (uc *AddressUseCase) Delete(ctx context.Context, userID, addressID string) error {
	existing, err := uc.addresses.Addresses(ctx, userID)
	if err != nil {
		return errs.Wrap(err, errs.KindUnavailable, "addresses_unavailable",
			"We could not remove that address just now. Please try again.")
	}

	remaining := make([]domain.Address, 0, len(existing))
	found := false
	for _, a := range existing {
		if a.ID == addressID {
			found = true
			continue
		}
		remaining = append(remaining, a)
	}
	if !found {
		return notFound(addressID)
	}

	if err := uc.addresses.DeleteAddress(ctx, userID, addressID); err != nil {
		return errs.Wrap(err, errs.KindUnavailable, "address_write_failed",
			"We could not remove that address just now. Please try again.")
	}

	// ChooseDefault decides who inherits. Writing the whole remaining list
	// rather than patching one row keeps "exactly one default" true at every
	// moment a concurrent reader could observe.
	if len(remaining) > 0 {
		if err := uc.save(ctx, userID, domain.ChooseDefault(remaining)); err != nil {
			return err
		}
	}
	return nil
}

// SetDefault makes one address the default.
func (uc *AddressUseCase) SetDefault(ctx context.Context, userID, addressID string) (domain.Address, error) {
	existing, err := uc.addresses.Addresses(ctx, userID)
	if err != nil {
		return domain.Address{}, errs.Wrap(err, errs.KindUnavailable, "addresses_unavailable",
			"We could not update your addresses just now. Please try again.")
	}

	index := -1
	for i := range existing {
		if existing[i].ID == addressID {
			index = i
			break
		}
	}
	if index == -1 {
		return domain.Address{}, notFound(addressID)
	}

	updated := promote(existing, addressID)
	if err := uc.save(ctx, userID, updated); err != nil {
		return domain.Address{}, err
	}
	// promote preserves ids and the address was found above, so the index is
	// still valid. Returning it directly rather than searching again avoids a
	// not-found branch that nothing could reach.
	return updated[index], nil
}

// Default returns the user's default address.
func (uc *AddressUseCase) Default(ctx context.Context, userID string) (domain.Address, error) {
	address, err := uc.addresses.DefaultAddress(ctx, userID)
	if errors.Is(err, domain.ErrNoDefaultAddress) {
		return domain.Address{}, errs.Wrap(err, errs.KindNotFound, "no_default_address",
			"Please add a delivery address.")
	}
	if err != nil {
		return domain.Address{}, errs.Wrap(err, errs.KindUnavailable, "addresses_unavailable",
			"We could not load your address just now. Please try again.")
	}
	return address, nil
}

// build validates a request into an address.
func (uc *AddressUseCase) build(addressID, userID string, req AddressRequest) (domain.Address, error) {
	pin, err := domain.NewPin(req.Lat, req.Lng)
	if err != nil {
		return domain.Address{}, errs.Wrap(err, errs.KindInvalid, "invalid_pin",
			"Please drop the pin on the delivery location.")
	}

	address, err := domain.NewAddress(addressID, userID, req.Label,
		req.RecipientName, req.RecipientPhone, req.Line1, req.Line2, req.Instructions, pin)
	if err != nil {
		return domain.Address{}, addressError(err)
	}
	return address, nil
}

// place resolves an address to its administrative area.
func (uc *AddressUseCase) place(ctx context.Context, address domain.Address) (domain.Address, error) {
	area, err := uc.geo.ResolveArea(ctx, geoext.Point{Lat: address.Pin.Lat, Lng: address.Pin.Lng})
	if err != nil {
		// The geo module already distinguishes "outside the service area" from
		// "we could not check", and both messages are written for a customer.
		// Passing the error through keeps one wording rather than inventing a
		// second here.
		return domain.Address{}, err
	}
	return address.WithPlacement(domain.Placement{
		AreaCode:     area.AreaCode,
		AreaName:     area.AreaName,
		DistrictCode: area.DistrictCode,
		DivisionCode: area.DivisionCode,
	}), nil
}

// save writes the whole list.
func (uc *AddressUseCase) save(ctx context.Context, userID string, addresses []domain.Address) error {
	if err := uc.addresses.SaveAddresses(ctx, userID, domain.ChooseDefault(addresses)); err != nil {
		return errs.Wrap(err, errs.KindUnavailable, "address_write_failed",
			"We could not save that address just now. Please try again.")
	}
	return nil
}

// promote marks one address as the default and clears the rest.
func promote(addresses []domain.Address, addressID string) []domain.Address {
	out := make([]domain.Address, len(addresses))
	copy(out, addresses)
	for i := range out {
		out[i].IsDefault = out[i].ID == addressID
	}
	return out
}

func notFound(addressID string) error {
	return errs.Wrap(domain.ErrAddressNotFound, errs.KindNotFound, "address_not_found",
		"We could not find that address.").With("address_id", addressID)
}

// addressError turns a domain refusal into something a person can act on.
//
// Pin failures are not listed: build validates the pin through NewPin before
// NewAddress ever sees it, so the pin check inside NewAddress cannot fire from
// this path. Listing it would be a branch no test could reach.
func addressError(err error) error {
	switch {
	case errors.Is(err, domain.ErrEmptyAddressLine):
		return errs.Wrap(err, errs.KindInvalid, "address_line_required",
			"Please enter the street address.")
	case errors.Is(err, domain.ErrMissingRecipient):
		return errs.Wrap(err, errs.KindInvalid, "recipient_required",
			"Please give a name and phone number for whoever will receive the delivery.")
	case errors.Is(err, domain.ErrAddressLineTooLong), errors.Is(err, domain.ErrLabelTooLong),
		errors.Is(err, domain.ErrNameTooLong):
		return errs.Wrap(err, errs.KindInvalid, "address_too_long",
			"Some of those details are too long.")
	default:
		return errs.Wrap(err, errs.KindInvalid, "invalid_address",
			"Some of those details are not valid.")
	}
}
