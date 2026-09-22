package domain

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// Errors returned when building an address.
var (
	// ErrEmptyAddressLine means the street address is missing.
	ErrEmptyAddressLine = errors.New("street address is required")
	// ErrAddressLineTooLong means a line exceeds the limit.
	ErrAddressLineTooLong = errors.New("address line is too long")
	// ErrLabelTooLong means the label exceeds the limit.
	ErrLabelTooLong = errors.New("address label is too long")
	// ErrMissingRecipient means the delivery contact is incomplete.
	ErrMissingRecipient = errors.New("a recipient name and phone are required")
	// ErrUnplaced means the address has not been resolved to an area.
	ErrUnplaced = errors.New("this address has not been placed in a service area")
	// ErrAddressNotFound means no such address belongs to this user.
	ErrAddressNotFound = errors.New("address not found")
	// ErrNoDefaultAddress means the user has none.
	ErrNoDefaultAddress = errors.New("no default address")
	// ErrInvalidPin means the map pin is not a usable coordinate.
	ErrInvalidPin = errors.New("that map location is not valid")
)

const (
	maxLabelLength       = 40
	maxAddressLineLength = 200
	maxInstructionLength = 300
)

// Pin is where the rider actually goes.
//
// Latitude and longitude are validated here rather than deferred to the geo
// module, because an address stored with a bad pin is an order that cannot be
// delivered — and the failure would surface at dispatch, hours later, to
// somebody who cannot fix it.
type Pin struct {
	Lat float64
	Lng float64
}

// NewPin validates a map location.
func NewPin(lat, lng float64) (Pin, error) {
	if lat < -90 || lat > 90 {
		return Pin{}, fmt.Errorf("%w: latitude %v", ErrInvalidPin, lat)
	}
	if lng < -180 || lng > 180 {
		return Pin{}, fmt.Errorf("%w: longitude %v", ErrInvalidPin, lng)
	}
	// A pin at exactly (0,0) is in the Gulf of Guinea and is almost always an
	// uninitialised variable rather than a place someone lives.
	if lat == 0 && lng == 0 {
		return Pin{}, fmt.Errorf("%w: the pin was not set", ErrInvalidPin)
	}
	return Pin{Lat: lat, Lng: lng}, nil
}

// Placement is where an address sits administratively.
//
// It is resolved from the pin by the geo module and stored, rather than being
// recomputed on every read: config resolution, pricing and dispatch all key off
// it, and re-deriving it per request would put a spatial query on the hot path
// of every order.
type Placement struct {
	AreaCode     string
	AreaName     string
	DistrictCode string
	DivisionCode string
}

// IsPlaced reports whether the address has been located.
func (p Placement) IsPlaced() bool { return p.DivisionCode != "" }

// Address is somewhere a customer wants a delivery taken.
type Address struct {
	ID     string
	UserID string
	// Label is what the customer calls it — Home, Office, or anything else.
	Label string
	// Recipient may differ from the account holder: people send food to their
	// parents, and the rider needs the name and number of whoever will open the
	// door.
	RecipientName  string
	RecipientPhone string
	Line1          string
	Line2          string
	// Instructions are free text for the rider: "third floor, blue gate".
	Instructions string
	Pin          Pin
	Placement    Placement
	IsDefault    bool
}

// NewAddress validates and builds an address.
//
// The placement is not required here: an address is built from what the
// customer typed, and placing it is a separate step that can fail for reasons
// the customer cannot fix. Saving it unplaced is refused at the use case
// instead, where there is something useful to say about it.
func NewAddress(id, userID, label, recipientName, recipientPhone, line1, line2, instructions string, pin Pin) (Address, error) {
	if strings.TrimSpace(userID) == "" {
		return Address{}, ErrEmptyUserID
	}

	label = strings.TrimSpace(label)
	if utf8.RuneCountInString(label) > maxLabelLength {
		return Address{}, fmt.Errorf("%w: limit %d characters", ErrLabelTooLong, maxLabelLength)
	}

	line1 = strings.TrimSpace(line1)
	if line1 == "" {
		return Address{}, ErrEmptyAddressLine
	}
	line2 = strings.TrimSpace(line2)
	for _, line := range []string{line1, line2} {
		if utf8.RuneCountInString(line) > maxAddressLineLength {
			return Address{}, fmt.Errorf("%w: limit %d characters",
				ErrAddressLineTooLong, maxAddressLineLength)
		}
	}

	recipientName = strings.TrimSpace(recipientName)
	recipientPhone = strings.TrimSpace(recipientPhone)
	if recipientName == "" || recipientPhone == "" {
		return Address{}, ErrMissingRecipient
	}
	if utf8.RuneCountInString(recipientName) > maxNameLength {
		return Address{}, fmt.Errorf("%w: limit %d characters", ErrNameTooLong, maxNameLength)
	}

	instructions = strings.TrimSpace(instructions)
	if utf8.RuneCountInString(instructions) > maxInstructionLength {
		return Address{}, fmt.Errorf("%w: limit %d characters",
			ErrAddressLineTooLong, maxInstructionLength)
	}

	if pin == (Pin{}) {
		return Address{}, fmt.Errorf("%w: a map location is required", ErrInvalidPin)
	}

	return Address{
		ID:             id,
		UserID:         userID,
		Label:          label,
		RecipientName:  recipientName,
		RecipientPhone: recipientPhone,
		Line1:          line1,
		Line2:          line2,
		Instructions:   instructions,
		Pin:            pin,
	}, nil
}

// WithPlacement returns a copy located in a service area.
func (a Address) WithPlacement(p Placement) Address {
	a.Placement = p
	return a
}

// DisplayLabel returns the label, or a sensible stand-in.
func (a Address) DisplayLabel() string {
	if a.Label != "" {
		return a.Label
	}
	return a.Line1
}

// SingleLine renders the address for a receipt or a rider's screen.
//
// The server composes it so every surface shows the same string. A client that
// joins the parts itself will eventually join them differently, and a customer
// comparing the app with an SMS should not see two addresses.
func (a Address) SingleLine() string {
	parts := make([]string, 0, 3)
	parts = append(parts, a.Line1)
	if a.Line2 != "" {
		parts = append(parts, a.Line2)
	}
	if a.Placement.AreaName != "" {
		parts = append(parts, a.Placement.AreaName)
	}
	return strings.Join(parts, ", ")
}

// ChooseDefault decides which address is the default after a change.
//
// Exactly one address is the default, and a user who has any addresses has a
// default. The rule is here rather than in SQL because "what happens when you
// delete the default" is a product decision, and burying it in a trigger is how
// it ends up answered differently by the next person who touches the schema.
//
// The newest remaining address wins: someone who deletes their default is
// usually moving on from it, and the most recently added is the best guess at
// where they are now.
func ChooseDefault(addresses []Address) []Address {
	if len(addresses) == 0 {
		return addresses
	}

	out := make([]Address, len(addresses))
	copy(out, addresses)

	explicit := -1
	for i := range out {
		if out[i].IsDefault {
			explicit = i
			break
		}
	}
	if explicit == -1 {
		explicit = len(out) - 1
	}

	for i := range out {
		out[i].IsDefault = i == explicit
	}
	return out
}
