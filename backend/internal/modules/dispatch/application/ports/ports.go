// Package ports declares what the dispatch use cases need from the outside
// world. No use case ever touches a database handle (05-architecture.md 2.3).
package ports

import (
	"context"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/dispatch/domain"
)

// PartnerRepository stores the people who carry orders.
type PartnerRepository interface {
	// Partner returns one partner by their own id.
	Partner(ctx context.Context, partnerID string) (domain.Partner, error)

	// PartnerOfUser returns the partner record for an account. The second
	// return is false when the account is not a partner, which is most
	// accounts.
	PartnerOfUser(ctx context.Context, userID string) (domain.Partner, bool, error)

	// SavePartner writes a partner, creating them if absent.
	SavePartner(ctx context.Context, p domain.Partner) error

	// AvailableWithin returns the partners who are working, have room, and are
	// inside radiusM of a point — the candidate pool for one assignment round.
	//
	// The spatial predicate is in the query rather than a filter afterwards: a
	// dispatch sweep in Dhaka would otherwise pull every partner in the city
	// into memory to throw nearly all of them away.
	AvailableWithin(ctx context.Context, at domain.Place, radiusM float64, band domain.Band, limit int) ([]PartnerDistance, error)

	// RecordOffer notes that a partner was offered a job, for the acceptance
	// rate. Separate from SavePartner because it is a counter increment and a
	// read-modify-write would lose offers made concurrently.
	RecordOffer(ctx context.Context, partnerID string, accepted bool) error
}

// PartnerDistance is a candidate partner with how far they are from the pickup.
type PartnerDistance struct {
	Partner   domain.Partner
	DistanceM float64
}

// JobFilter narrows a listing of jobs.
type JobFilter struct {
	// PartnerID scopes to one partner's own jobs.
	PartnerID string
	// LiveOnly is the "current work" list.
	LiveOnly bool
	// Statuses narrows to particular states. Empty means all.
	Statuses []domain.JobStatus
	Limit    int
	Offset   int
}

// JobRepository stores deliveries.
type JobRepository interface {
	// CreateJob writes a new job. Refused when the order already has one, by a
	// unique index rather than a check — an order that becomes ready twice
	// must not become two jobs for two riders.
	CreateJob(ctx context.Context, j domain.Job) error

	// Job returns one delivery.
	Job(ctx context.Context, jobID string) (domain.Job, error)

	// JobForOrder returns the delivery for an order, if there is one.
	JobForOrder(ctx context.Context, orderID string) (domain.Job, bool, error)

	// Jobs lists deliveries matching a filter, with the total before paging.
	Jobs(ctx context.Context, f JobFilter) ([]domain.Job, int, error)

	// SaveJob writes a job, checking it is still where the caller thought it
	// was.
	//
	// This is the other half of "two partners cannot be assigned the same
	// order": the compare-and-set is in the UPDATE's own WHERE clause, so two
	// riders accepting the same waiting job in the same millisecond cannot both
	// succeed. The domain refuses a partner moving a job they do not hold; this
	// refuses the race that would give them both the same claim.
	SaveJob(ctx context.Context, j domain.Job, expected domain.JobStatus) error

	// WaitingNear returns the waiting jobs whose pickup is inside radiusM of a
	// point — the raw material of a partner's feed (ALG-08).
	WaitingNear(ctx context.Context, at domain.Place, radiusM float64, limit int) ([]JobDistance, error)

	// LapsedOffers returns jobs whose offer has run out, for the sweeper.
	LapsedOffers(ctx context.Context, limit int) ([]domain.Job, error)

	// WaitingJobs returns jobs nobody holds, oldest first, for the sweeper's
	// second pass. Oldest first because the customer who has been waiting
	// longest is the one most likely to give up.
	WaitingJobs(ctx context.Context, limit int) ([]domain.Job, error)
}

// JobDistance is a job with how far its pickup is from the asker.
type JobDistance struct {
	Job       domain.Job
	DistanceM float64
}

// Repository is the whole dispatch store.
type Repository interface {
	PartnerRepository
	JobRepository
}
