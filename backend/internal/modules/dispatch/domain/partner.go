// Package domain holds the rules of getting an order from a counter to a door:
// who may carry it, which jobs they are shown, and who gets offered what. It
// imports nothing outside itself.
//
// D4 is the shape of it. A delivery partner chooses between long-distance and
// short-distance work, and is shown only the jobs inside their own radius —
// so the feed is not "every job" filtered by the app, it is a different list
// for every partner, computed here.
package domain

import "errors"

// Availability is whether a partner is taking work.
type Availability string

// The three states a partner can be in.
const (
	// AvailabilityOffline is not working. No job is ever offered.
	AvailabilityOffline Availability = "offline"
	// AvailabilityAvailable is working and able to take more.
	AvailabilityAvailable Availability = "available"
	// AvailabilityBusy is working and at their concurrent limit. Distinct from
	// offline because it ends by itself — a partner who finishes a delivery
	// becomes available again without touching anything.
	AvailabilityBusy Availability = "busy"
)

// Preference is D4's choice: what kind of distance a partner wants.
type Preference string

// The three answers a partner can give.
const (
	// PreferenceShort is local work only — a bicycle, a neighbourhood.
	PreferenceShort Preference = "short"
	// PreferenceLong is intercity and long-haul work.
	PreferenceLong Preference = "long"
	// PreferenceAny is both, which is the default: a partner who has not
	// chosen has not chosen to exclude anything.
	PreferenceAny Preference = "any"
)

// Valid reports whether a string names a preference.
func (p Preference) Valid() bool {
	return p == PreferenceShort || p == PreferenceLong || p == PreferenceAny
}

// Band is how far a job is, in D4's terms.
type Band string

// The distance bands, from dispatch.short_distance_max and
// dispatch.long_distance_max (Appendix B).
const (
	// BandShort is within the short-distance ceiling.
	BandShort Band = "short"
	// BandLong is beyond it, up to the long-distance ceiling.
	BandLong Band = "long"
	// BandBeyond is past even the long ceiling. Such a job exists — D3 allows
	// a delivery across a whole division — but no partner's preference covers
	// it, so it is offered to everybody working rather than to nobody.
	BandBeyond Band = "beyond"
)

// BandFor places a distance in its band.
func BandFor(distanceM, shortMaxM, longMaxM float64) Band {
	switch {
	case distanceM <= shortMaxM:
		return BandShort
	case distanceM <= longMaxM:
		return BandLong
	default:
		return BandBeyond
	}
}

// Accepts reports whether a partner wants a job in this band — D4's choice,
// applied.
//
// A job past the long ceiling is offered to everyone rather than to no one. The
// alternative is an order that reaches `ready` and sits there because nobody's
// preference covers it, which is worse for the customer, the shop and the
// partner who would happily have taken it.
func (p Preference) Accepts(b Band) bool {
	switch b {
	case BandShort:
		return p == PreferenceShort || p == PreferenceAny
	case BandLong:
		return p == PreferenceLong || p == PreferenceAny
	default:
		return true
	}
}

// The rules a partner holds itself to.
var (
	ErrNoUser          = errors.New("a partner must belong to an account")
	ErrNoName          = errors.New("a partner must have a name")
	ErrNoPhone         = errors.New("a partner must have a phone number")
	ErrBadPreference   = errors.New("that is not a distance choice")
	ErrBadAvailability = errors.New("that is not an availability")
	ErrNotWorking      = errors.New("that partner is not taking work")
	ErrAtCapacity      = errors.New("that partner already has as many jobs as they can carry")
)

// Partner is somebody who carries orders.
type Partner struct {
	ID     string
	UserID string
	Name   string
	Phone  string
	// Vehicle is free text — "bicycle", "motorcycle", "van". Not an enum,
	// because the list would be wrong within a month in a country where people
	// deliver on whatever they have.
	Vehicle string

	Availability Availability
	Preference   Preference

	// Lat and Lng are where they last reported being. The feed is computed
	// from here, so a partner whose app has not reported in a while sees a
	// stale list — which is why the transport updates it on every feed read.
	Lat float64
	Lng float64

	// Carrying is how many live jobs they hold. Compared against
	// dispatch.max_concurrent_jobs, which is configuration rather than a
	// property of the person.
	Carrying int

	// Offered and Accepted count the offers this partner has been made and
	// taken. Their ratio feeds the assignment score (ALG-04): a partner who
	// declines everything makes every customer wait for the next offer round.
	Offered  int
	Accepted int
}

// NewPartner registers somebody to carry orders.
//
// They start offline and taking any distance. Offline because signing up is not
// the same as starting a shift, and any because a partner who has not chosen
// has not chosen to exclude anything.
func NewPartner(id, userID, name, phone, vehicle string) (Partner, error) {
	switch {
	case userID == "":
		return Partner{}, ErrNoUser
	case name == "":
		return Partner{}, ErrNoName
	case phone == "":
		return Partner{}, ErrNoPhone
	}
	return Partner{
		ID: id, UserID: userID, Name: name, Phone: phone, Vehicle: vehicle,
		Availability: AvailabilityOffline, Preference: PreferenceAny,
	}, nil
}

// AcceptanceRate is the share of offers this partner has taken, 0 to 1.
//
// A partner with no offers yet scores as 1 rather than 0. The alternative
// punishes somebody for being new, which is the surest way to have no new
// partners: their first offer would go to the bottom of every heap.
func (p Partner) AcceptanceRate() float64 {
	if p.Offered <= 0 {
		return 1
	}
	rate := float64(p.Accepted) / float64(p.Offered)
	if rate > 1 {
		return 1
	}
	return rate
}

// CanTake reports whether a partner can be offered another job now.
func (p Partner) CanTake(maxConcurrent int) error {
	if p.Availability != AvailabilityAvailable {
		return ErrNotWorking
	}
	if maxConcurrent > 0 && p.Carrying >= maxConcurrent {
		return ErrAtCapacity
	}
	return nil
}

// WithAvailability sets whether a partner is working.
//
// Busy is not something a partner declares: it is what being at the concurrent
// limit is called, and it resolves itself when they finish a delivery. Letting
// somebody set it by hand would give them a way to stay in the pool while
// refusing every offer.
func (p Partner) WithAvailability(a Availability) (Partner, error) {
	if a != AvailabilityOffline && a != AvailabilityAvailable {
		return Partner{}, ErrBadAvailability
	}
	p.Availability = a
	return p, nil
}

// WithPreference sets D4's distance choice.
func (p Partner) WithPreference(pref Preference) (Partner, error) {
	if !pref.Valid() {
		return Partner{}, ErrBadPreference
	}
	p.Preference = pref
	return p, nil
}

// At records where the partner is.
func (p Partner) At(lat, lng float64) Partner {
	p.Lat, p.Lng = lat, lng
	return p
}
