package application

import (
	"context"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/dispatch/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/dispatch/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/dispatch/domain"
	cfg "github.com/rootlogic-lab/delivery/backend/internal/modules/dispatch/external/config"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/clock"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/id"
)

// candidatePool caps how many partners one assignment round considers.
//
// Fifty is far more than any radius realistically holds and bounds the query
// against the day one does. The heap is O(n log n) either way; the cost being
// avoided is pulling a city's worth of riders into memory to throw nearly all
// of them away.
const candidatePool = 50

// OfferUseCase puts a delivery on the board and decides who to ask.
//
// Offering is a round, not a broadcast: one partner at a time, each with a
// clock. Broadcasting would be simpler and would have five riders race to the
// same counter, four of whom wasted a trip — which is how a platform loses
// riders.
type OfferUseCase struct {
	repo   ports.Repository
	config cfg.Service
	clock  clock.Clock
	ids    id.Generator
}

// NewOfferUseCase wires the use case.
func NewOfferUseCase(repo ports.Repository, config cfg.Service, c clock.Clock, ids id.Generator) *OfferUseCase {
	return &OfferUseCase{repo: repo, config: config, clock: c, ids: ids}
}

// Offer puts an order on the board, and offers it to the best partner nearby.
//
// Idempotent on the order id. An order that reaches `ready` twice — a retry, a
// re-delivered event — must not become two jobs for two riders, and the unique
// index behind JobForOrder is what makes that true rather than a check that
// races with itself.
func (uc *OfferUseCase) Offer(ctx context.Context, req contract.OfferRequest) (contract.Job, error) {
	existing, found, err := uc.repo.JobForOrder(ctx, req.OrderID)
	if err != nil {
		return contract.Job{}, storageError(err)
	}
	if found {
		return uc.toContract(ctx, existing), nil
	}

	placement := domain.Placement{
		AreaCode:     req.AreaCode,
		DistrictCode: req.DistrictCode,
		DivisionCode: req.DivisionCode,
	}
	settings, err := resolveRules(ctx, uc.config, placementOf(placement))
	if err != nil {
		return contract.Job{}, err
	}

	now := uc.clock.Now()
	job, err := domain.NewJob(
		uc.ids.New("JOB"), req.OrderID, req.Code,
		placeOf(req.Pickup), placeOf(req.Destination),
		req.DistanceM,
		domain.BandFor(req.DistanceM, settings.shortMaxM, settings.longMaxM),
		placement,
		now,
	)
	if err != nil {
		return contract.Job{}, jobError(err)
	}
	if err := uc.repo.CreateJob(ctx, job); err != nil {
		// The unique index caught a second "ready" for this order that raced
		// the check above. The job that won is the answer; this one never
		// existed.
		if errs.CodeOf(err) == "job_exists" {
			won, found, readErr := uc.repo.JobForOrder(ctx, req.OrderID)
			if readErr == nil && found {
				return uc.toContract(ctx, won), nil
			}
		}
		return contract.Job{}, storageError(err)
	}

	// The first offer round runs here rather than on a timer, so a rider's
	// phone buzzes while the food is still hot. A job nobody takes falls back
	// to waiting and the sweeper picks it up.
	offered, err := uc.Round(ctx, job, settings)
	if err != nil {
		// The job exists and is on the board; failing to find somebody for it
		// right now is not a reason to tell the shop the delivery could not be
		// arranged. It waits, and the next round tries again.
		return uc.toContract(ctx, job), nil
	}
	return uc.toContract(ctx, offered), nil
}

// Round offers a waiting job to the best partner available (ALG-04).
//
// Returns the job unchanged when there is nobody to ask — which is a normal
// state in a rural area at two in the morning, not a failure.
func (uc *OfferUseCase) Round(ctx context.Context, job domain.Job, settings rules) (domain.Job, error) {
	candidates, err := uc.repo.AvailableWithin(ctx, job.Pickup, settings.partnerRadius, job.Band, candidatePool)
	if err != nil {
		return job, storageError(err)
	}
	if len(candidates) == 0 {
		return job, nil
	}

	pool := make([]domain.Candidate, 0, len(candidates))
	// everybody is the same pool with the rider who just passed put back. The
	// rider who declined this job, or let its offer run out, is not asked again
	// while anybody else is standing by — asking twice is how a feed becomes
	// noise a partner learns to ignore. But in a town with one rider they are
	// the only way the order gets delivered, and a job nobody can ever be
	// offered again is worse than a repeated question.
	everybody := make([]domain.Candidate, 0, len(candidates))
	for _, c := range candidates {
		if c.Partner.CanTake(settings.maxConcurrent) != nil {
			continue
		}
		candidate := domain.Candidate{
			PartnerID:      c.Partner.ID,
			DistanceM:      c.DistanceM,
			Carrying:       c.Partner.Carrying,
			MaxConcurrent:  settings.maxConcurrent,
			AcceptanceRate: c.Partner.AcceptanceRate(),
		}
		everybody = append(everybody, candidate)
		if c.Partner.ID != job.PassedBy {
			pool = append(pool, candidate)
		}
	}
	if len(pool) == 0 {
		pool = everybody
	}
	if len(pool) == 0 {
		return job, nil
	}

	// The pool is not empty, so there is a best candidate.
	best, _ := domain.NewAssigner(pool, settings.partnerRadius).Next()

	now := uc.clock.Now()
	expected := job.Status
	if err := job.Offer(best.PartnerID, now, settings.offerTimeout); err != nil {
		return job, jobError(err)
	}
	if err := uc.repo.SaveJob(ctx, job, expected); err != nil {
		// Somebody else moved it between the read and the write. That is the
		// compare-and-set doing its job, not a fault.
		return job, storageError(err)
	}
	if err := uc.repo.RecordOffer(ctx, best.PartnerID, false); err != nil {
		// The offer stands; only the acceptance-rate counter is behind. A
		// delivery is not worth refusing over a statistic.
		_ = err
	}
	return job, nil
}

// Withdraw takes a job off the board because the order behind it went away.
func (uc *OfferUseCase) Withdraw(ctx context.Context, orderID, reason string) error {
	job, found, err := uc.repo.JobForOrder(ctx, orderID)
	if err != nil {
		return storageError(err)
	}
	if !found {
		// An order cancelled before it was ever ready never had a job. Not an
		// error: the caller's intent is satisfied.
		return nil
	}
	if job.Status.IsTerminal() {
		return nil
	}

	expected := job.Status
	held := job.PartnerID
	// Cancel's only failure is a terminal job, which the check above already
	// ruled out — its error is unreachable here and not worth a branch to
	// report.
	_ = job.Cancel(reason, uc.clock.Now())
	if err := uc.repo.SaveJob(ctx, job, expected); err != nil {
		return storageError(err)
	}
	if held != "" {
		uc.release(ctx, held)
	}
	return nil
}

// release frees the capacity a cancelled job was occupying.
func (uc *OfferUseCase) release(ctx context.Context, partnerID string) {
	partner, err := uc.repo.Partner(ctx, partnerID)
	if err != nil {
		return
	}
	if partner.Carrying > 0 {
		partner.Carrying--
	}
	if partner.Availability == domain.AvailabilityBusy {
		partner.Availability = domain.AvailabilityAvailable
	}
	_ = uc.repo.SavePartner(ctx, partner)
}

// SweepResult is what one sweep did.
type SweepResult struct {
	// Expired is offers nobody answered, now back on the board.
	Expired int
	// Offered is waiting jobs put to a partner this pass.
	Offered int
}

// Sweep is the dispatch heartbeat: expire offers nobody answered, then offer
// the waiting jobs to somebody.
//
// A separate pass rather than a check on read, because the customer whose order
// is sitting in an unanswered offer is not the one refreshing a screen. Both
// halves are needed: without the first a job stalls behind a rider who put
// their phone away, and without the second a job that was declined would sit
// waiting forever, since a decline is the one way onto the board that no
// partner's action takes it off again. Safe to run from anywhere and safe to
// run twice.
func (uc *OfferUseCase) Sweep(ctx context.Context, limit int) (SweepResult, error) {
	out := SweepResult{}

	lapsed, err := uc.repo.LapsedOffers(ctx, limit)
	if err != nil {
		return out, storageError(err)
	}
	now := uc.clock.Now()
	for _, job := range lapsed {
		expected := job.Status
		abandoned := job.PartnerID
		if err := job.Lapse(now); err != nil {
			continue
		}
		if err := uc.repo.SaveJob(ctx, job, expected); err != nil {
			// Somebody accepted it while the sweep was running. Good.
			continue
		}
		// An offer that lapsed counts against the partner's acceptance rate
		// exactly like a decline, because to the customer it was the same
		// thing: a rider who was asked and did not come.
		_ = uc.repo.RecordOffer(ctx, abandoned, false)
		out.Expired++
	}

	waiting, err := uc.repo.WaitingJobs(ctx, limit)
	if err != nil {
		return out, storageError(err)
	}
	for _, job := range waiting {
		settings, err := resolveRules(ctx, uc.config, placementOf(job.Placement))
		if err != nil {
			// One area with unreadable settings must not stop the sweep for
			// every other area.
			continue
		}
		offered, err := uc.Round(ctx, job, settings)
		if err != nil {
			continue
		}
		if offered.Status == domain.JobOffered {
			out.Offered++
		}
	}
	return out, nil
}

// toContract restates a job for another module.
func (uc *OfferUseCase) toContract(ctx context.Context, job domain.Job) contract.Job {
	out := contract.Job{
		ID: job.ID, OrderID: job.OrderID,
		Status: string(job.Status), Live: job.Status.IsLive(),
		Band: string(job.Band), DistanceM: job.DistanceM,
		Attempts: job.Attempts, CreatedAt: job.CreatedAt, UpdatedAt: job.UpdatedAt,
	}
	if job.PartnerID == "" {
		return out
	}
	partner, err := uc.repo.Partner(ctx, job.PartnerID)
	if err != nil {
		// A job whose partner cannot be read is still a job. The caller gets
		// the delivery without the rider's details rather than an outage.
		return out
	}
	out.Partner = contract.Partner{
		ID: partner.ID, Name: partner.Name, Phone: partner.Phone,
		Vehicle: partner.Vehicle, Lat: partner.Lat, Lng: partner.Lng,
	}
	return out
}

func placeOf(p contract.Place) domain.Place {
	return domain.Place{
		Name: p.Name, Phone: p.Phone, SingleLine: p.SingleLine, Lat: p.Lat, Lng: p.Lng,
	}
}

// placementOf restates a job's placement for the config module.
func placementOf(p domain.Placement) cfg.Placement {
	return cfg.Placement{
		AreaCode:     p.AreaCode,
		DistrictCode: p.DistrictCode,
		DivisionCode: p.DivisionCode,
	}
}
