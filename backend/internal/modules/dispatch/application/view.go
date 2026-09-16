package application

import (
	"strconv"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/dispatch/domain"
)

// Everything a rider's screen renders, already decided. A partner app is used
// one-handed on a motorbike at a traffic light; every string it shows is
// composed here so the app has nothing to work out and nothing to get wrong
// (2.9), and it is Bengali-first because riders are the audience (1.4).

// PartnerView is a partner's own record.
type PartnerView struct {
	ID      string
	Name    string
	Phone   string
	Vehicle string

	Availability      string
	AvailabilityLabel string
	Preference        string
	PreferenceLabel   string

	Lat      float64
	Lng      float64
	Carrying int
	// AcceptancePercent is the rate as a whole number, because a rider reading
	// "০.৮৩" learns less than one reading "৮৩%".
	AcceptancePercent int
}

func partnerView(p domain.Partner, lang string) PartnerView {
	return PartnerView{
		ID: p.ID, Name: p.Name, Phone: p.Phone, Vehicle: p.Vehicle,
		Availability:      string(p.Availability),
		AvailabilityLabel: availabilityLabel(p.Availability, lang),
		Preference:        string(p.Preference),
		PreferenceLabel:   preferenceLabel(p.Preference, lang),
		Lat:               p.Lat, Lng: p.Lng, Carrying: p.Carrying,
		AcceptancePercent: int(p.AcceptanceRate()*100 + 0.5),
	}
}

// PlaceView is one end of a delivery, as the rider's screen shows it.
type PlaceView struct {
	Name       string
	Phone      string
	SingleLine string
	Lat        float64
	Lng        float64
}

// JobView is one delivery.
type JobView struct {
	ID      string
	OrderID string
	// Code is what the rider says to the shopkeeper.
	Code string

	Status      string
	StatusLabel string
	Live        bool

	Band      string
	BandLabel string
	DistanceM float64
	// Distance is the delivery distance in words — pickup to door.
	Distance string
	// ToPickup is how far the rider is from the counter, when the screen knows.
	ToPickupM float64
	ToPickup  string

	Pickup      PlaceView
	Destination PlaceView

	Reason string
	// SecondsLeft is how long is left to answer an offer. Zero on anything
	// that is not an open offer.
	SecondsLeft int
}

// FeedView is the rider's list of available work.
type FeedView struct {
	Partner PartnerView
	Jobs    []JobView
	RadiusM float64
	// Reason and Notice say why the list is empty, when it is. "Nothing here"
	// and "you are offline" are different facts and a rider waiting for work
	// deserves the right one.
	Reason string
	Notice string
}

// JobListView is a page of a rider's own jobs.
type JobListView struct {
	Jobs  []JobView
	Total int
}

// The reasons a feed can be empty.
const (
	feedOffline       = "offline"
	feedAtCapacity    = "at_capacity"
	feedNothingNearby = "nothing_nearby"
)

func jobView(job domain.Job, toPickupM float64, lang string) JobView {
	view := JobView{
		ID: job.ID, OrderID: job.OrderID, Code: job.Code,
		Status: string(job.Status), StatusLabel: jobStatusLabel(job.Status, lang),
		Live: job.Status.IsLive(),
		Band: string(job.Band), BandLabel: bandLabel(job.Band, lang),
		DistanceM: job.DistanceM, Distance: formatDistance(job.DistanceM, lang),
		ToPickupM: toPickupM, ToPickup: formatDistance(toPickupM, lang),
		Pickup: PlaceView{
			Name: job.Pickup.Name, Phone: job.Pickup.Phone,
			SingleLine: job.Pickup.SingleLine, Lat: job.Pickup.Lat, Lng: job.Pickup.Lng,
		},
		Destination: PlaceView{
			Name: job.Destination.Name, Phone: job.Destination.Phone,
			SingleLine: job.Destination.SingleLine,
			Lat:        job.Destination.Lat, Lng: job.Destination.Lng,
		},
		Reason: job.Reason,
	}
	if job.Status == domain.JobOffered && !job.OfferExpiresAt.IsZero() {
		// Composed from the server's clock. A rider's phone clock is wrong
		// often enough that a countdown from it would expire at the wrong
		// moment, which on an offer is the difference between a job and a
		// wasted trip.
		view.SecondsLeft = secondsUntil(job.OfferExpiresAt, job.UpdatedAt)
	}
	return view
}

// secondsUntil is how long is left, never negative.
func secondsUntil(deadline, now time.Time) int {
	left := int(deadline.Sub(now).Seconds())
	if left < 0 {
		return 0
	}
	return left
}

// ------------------------------------------------------------------ words

// bengaliDigits maps an ASCII digit to its Bengali form.
var bengaliDigits = [10]string{"০", "১", "২", "৩", "৪", "৫", "৬", "৭", "৮", "৯"}

func toBengaliDigits(s string) string {
	out := make([]byte, 0, len(s)*3)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= '0' && c <= '9' {
			out = append(out, bengaliDigits[c-'0']...)
			continue
		}
		out = append(out, c)
	}
	return string(out)
}

// formatDistance renders a distance for a rider.
//
// Coarser than the customer's version on purpose: a rider deciding whether to
// take a job wants "৩ কিমি", not "২.৮ কিমি". Below a kilometre it is metres
// rounded to a hundred, because that is the accuracy of a straight line through
// a city anyway.
func formatDistance(metres float64, lang string) string {
	bengali := lang != "en"
	if metres <= 0 {
		return ""
	}
	if metres < 1000 {
		rounded := int(metres/100+0.5) * 100
		if rounded == 0 {
			rounded = 100
		}
		if bengali {
			return toBengaliDigits(strconv.Itoa(rounded)) + " মিটার"
		}
		return strconv.Itoa(rounded) + " m"
	}
	km := strconv.FormatFloat(metres/1000, 'f', 1, 64)
	if bengali {
		return toBengaliDigits(km) + " কিমি"
	}
	return km + " km"
}

func availabilityLabel(a domain.Availability, lang string) string {
	bengali := lang != "en"
	switch a {
	case domain.AvailabilityAvailable:
		if bengali {
			return "কাজের জন্য প্রস্তুত"
		}
		return "Available"
	case domain.AvailabilityBusy:
		if bengali {
			return "ডেলিভারিতে ব্যস্ত"
		}
		return "On a delivery"
	default:
		if bengali {
			return "অফলাইন"
		}
		return "Offline"
	}
}

func preferenceLabel(p domain.Preference, lang string) string {
	bengali := lang != "en"
	switch p {
	case domain.PreferenceShort:
		if bengali {
			return "কাছের ডেলিভারি"
		}
		return "Short distance"
	case domain.PreferenceLong:
		if bengali {
			return "দূরের ডেলিভারি"
		}
		return "Long distance"
	default:
		if bengali {
			return "সব ধরনের ডেলিভারি"
		}
		return "Any distance"
	}
}

func bandLabel(b domain.Band, lang string) string {
	bengali := lang != "en"
	switch b {
	case domain.BandShort:
		if bengali {
			return "কাছে"
		}
		return "Nearby"
	case domain.BandLong:
		if bengali {
			return "দূরে"
		}
		return "Long distance"
	default:
		if bengali {
			return "অনেক দূরে"
		}
		return "Very long distance"
	}
}

func jobStatusLabel(s domain.JobStatus, lang string) string {
	bengali := lang != "en"
	switch s {
	case domain.JobWaiting:
		if bengali {
			return "রাইডারের অপেক্ষায়"
		}
		return "Waiting for a rider"
	case domain.JobOffered:
		if bengali {
			return "আপনাকে দেওয়া হয়েছে"
		}
		return "Offered to you"
	case domain.JobAssigned:
		if bengali {
			return "দোকান থেকে নিতে যান"
		}
		return "Collect from the shop"
	case domain.JobCollected:
		if bengali {
			return "পথে আছে"
		}
		return "On the way"
	case domain.JobDelivered:
		if bengali {
			return "পৌঁছে দেওয়া হয়েছে"
		}
		return "Delivered"
	case domain.JobFailed:
		if bengali {
			return "পৌঁছে দেওয়া যায়নি"
		}
		return "Could not be delivered"
	default:
		if bengali {
			return "বাতিল হয়েছে"
		}
		return "Cancelled"
	}
}

// feedNotice says why a rider's list is empty.
func feedNotice(reason, lang string) string {
	bengali := lang != "en"
	switch reason {
	case feedOffline:
		if bengali {
			return "আপনি অফলাইনে আছেন। কাজ পেতে অনলাইনে আসুন।"
		}
		return "You are offline. Go online to see deliveries."
	case feedAtCapacity:
		if bengali {
			return "আপনার হাতে এখন যতগুলো নেওয়া যায় ততগুলো ডেলিভারি আছে।"
		}
		return "You already have as many deliveries as you can carry."
	default:
		if bengali {
			return "আপনার আশেপাশে এখন কোনো ডেলিভারি নেই।"
		}
		return "There are no deliveries near you right now."
	}
}
