// Package domain holds review's own rules: what a rating needs to be valid,
// what a support ticket needs to be raised, and how an agent resolves one.
package domain

import (
	"errors"
	"time"
)

// Subject is what a review is about.
type Subject string

// The three things a customer may rate. Nothing else — an order itself is
// not a subject, because "how was your order" is really three separate
// opinions about three different parties, and merging them into one number
// would tell a shop nothing about whether a late delivery was its fault or
// the rider's.
const (
	SubjectMerchant Subject = "merchant"
	SubjectPartner  Subject = "partner"
	SubjectItem     Subject = "item"
)

// Valid reports whether s is one of the three subjects reviews are kept for.
func (s Subject) Valid() bool {
	switch s {
	case SubjectMerchant, SubjectPartner, SubjectItem:
		return true
	default:
		return false
	}
}

// Errors NewReview returns.
var (
	ErrNoOrder        = errors.New("a review needs the order it is about")
	ErrNoRater        = errors.New("a review needs who is rating")
	ErrInvalidSubject = errors.New("that is not something a review can be about")
	ErrNoSubjectID    = errors.New("a review needs what it is about")
	ErrInvalidRating  = errors.New("a rating must be between 1 and 5")
)

// Review is one customer's opinion of one thing, from one order.
type Review struct {
	ID        string
	OrderID   string
	RaterID   string
	Subject   Subject
	SubjectID string
	Rating    int
	Comment   string
	CreatedAt time.Time
}

// NewReview validates and builds a review. Eligibility — whether this rater
// may rate this subject from this order at all — is the use case's job, not
// this constructor's: the domain only knows a rating's own shape.
func NewReview(id, orderID, raterID string, subject Subject, subjectID string, rating int, comment string, now time.Time) (Review, error) {
	if orderID == "" {
		return Review{}, ErrNoOrder
	}
	if raterID == "" {
		return Review{}, ErrNoRater
	}
	if !subject.Valid() {
		return Review{}, ErrInvalidSubject
	}
	if subjectID == "" {
		return Review{}, ErrNoSubjectID
	}
	if rating < 1 || rating > 5 {
		return Review{}, ErrInvalidRating
	}
	return Review{
		ID: id, OrderID: orderID, RaterID: raterID,
		Subject: subject, SubjectID: subjectID,
		Rating: rating, Comment: comment, CreatedAt: now,
	}, nil
}

// Rating is one subject's reviews, reduced to what a screen shows: a mean
// out of five and how many opinions it rests on. A single five-star review
// and a hundred read very differently, and a screen that shows only the
// mean cannot tell them apart — Count travels with it so the caller can.
type Rating struct {
	Subject   Subject
	SubjectID string
	Average   float64
	Count     int
}
