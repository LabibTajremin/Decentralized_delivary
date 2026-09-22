package application

import (
	"context"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/dispatch/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/dispatch/domain"
	cfg "github.com/rootlogic-lab/delivery/backend/internal/modules/dispatch/external/config"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/dispatch/external/geo"
	orderx "github.com/rootlogic-lab/delivery/backend/internal/modules/dispatch/external/order"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/clock"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/id"
)

// feedFanOut caps how many waiting jobs the radius query pulls back before D4's
// preference filter runs. Larger than FeedLimit because the filter removes
// some, and bounded because the query is spatial.
const feedFanOut = 60

// PartnerUseCase is everything a delivery partner does.
type PartnerUseCase struct {
	repo   ports.Repository
	order  orderx.Service
	geo    geo.Service
	config cfg.Service
	clock  clock.Clock
	ids    id.Generator
}

// NewPartnerUseCase wires the use case.
func NewPartnerUseCase(
	repo ports.Repository,
	orders orderx.Service,
	geoService geo.Service,
	config cfg.Service,
	c clock.Clock,
	ids id.Generator,
) *PartnerUseCase {
	return &PartnerUseCase{repo: repo, order: orders, geo: geoService, config: config, clock: c, ids: ids}
}

// RegisterRequest is somebody signing up to carry orders.
type RegisterRequest struct {
	Name    string
	Phone   string
	Vehicle string
}

// Register signs an account up as a delivery partner.
//
// D1 again: a partner may provide delivery anywhere in Bangladesh, so there is
// no area to register in and nothing to approve against a map. Where they can
// work is decided every time they report a location.
func (uc *PartnerUseCase) Register(ctx context.Context, userID, lang string, req RegisterRequest) (PartnerView, error) {
	existing, found, err := uc.repo.PartnerOfUser(ctx, userID)
	if err != nil {
		return PartnerView{}, storageError(err)
	}
	if found {
		// Registering twice is not an error; it is somebody tapping a button
		// again. They get the partner they already are.
		return partnerView(existing, lang), nil
	}

	partner, err := domain.NewPartner(uc.ids.New("PTR"), userID, req.Name, req.Phone, req.Vehicle)
	if err != nil {
		return PartnerView{}, partnerError(err)
	}
	if err := uc.repo.SavePartner(ctx, partner); err != nil {
		return PartnerView{}, storageError(err)
	}
	return partnerView(partner, lang), nil
}

// Me returns the caller's own partner record.
func (uc *PartnerUseCase) Me(ctx context.Context, userID, lang string) (PartnerView, error) {
	partner, err := uc.mine(ctx, userID)
	if err != nil {
		return PartnerView{}, err
	}
	return partnerView(partner, lang), nil
}

// SetAvailability puts a partner on or off shift.
func (uc *PartnerUseCase) SetAvailability(ctx context.Context, userID, availability, lang string) (PartnerView, error) {
	partner, err := uc.mine(ctx, userID)
	if err != nil {
		return PartnerView{}, err
	}
	updated, err := partner.WithAvailability(domain.Availability(availability))
	if err != nil {
		return PartnerView{}, partnerError(err)
	}
	if err := uc.repo.SavePartner(ctx, updated); err != nil {
		return PartnerView{}, storageError(err)
	}
	return partnerView(updated, lang), nil
}

// SetPreference records D4's distance choice.
func (uc *PartnerUseCase) SetPreference(ctx context.Context, userID, preference, lang string) (PartnerView, error) {
	partner, err := uc.mine(ctx, userID)
	if err != nil {
		return PartnerView{}, err
	}
	updated, err := partner.WithPreference(domain.Preference(preference))
	if err != nil {
		return PartnerView{}, partnerError(err)
	}
	if err := uc.repo.SavePartner(ctx, updated); err != nil {
		return PartnerView{}, storageError(err)
	}
	return partnerView(updated, lang), nil
}

// ReportLocation records where a partner is.
//
// Called often — a rider's app reports as they move — so it writes one row and
// nothing else. The feed is computed on read rather than pushed on write,
// because a partner who is driving is not looking at their phone.
func (uc *PartnerUseCase) ReportLocation(ctx context.Context, userID string, lat, lng float64, lang string) (PartnerView, error) {
	partner, err := uc.mine(ctx, userID)
	if err != nil {
		return PartnerView{}, err
	}
	updated := partner.At(lat, lng)
	if err := uc.repo.SavePartner(ctx, updated); err != nil {
		return PartnerView{}, storageError(err)
	}
	return partnerView(updated, lang), nil
}

// Feed is ALG-08: the jobs this partner should see, nearest pickup first.
//
// The whole answer is computed here. A client that filtered a global list by
// its own radius would be deciding visibility, which 2.9 forbids, and would
// need a copy of dispatch.partner_radius to do it.
func (uc *PartnerUseCase) Feed(ctx context.Context, userID, lang string) (FeedView, error) {
	partner, err := uc.mine(ctx, userID)
	if err != nil {
		return FeedView{}, err
	}

	settings, err := uc.rulesAt(ctx, partner)
	if err != nil {
		return FeedView{}, err
	}

	// Offline is a real answer, not an empty list. A partner who forgot to go
	// online should be told that, not shown nothing and left to wonder.
	if partner.Availability == domain.AvailabilityOffline {
		return FeedView{
			Partner: partnerView(partner, lang),
			Notice:  feedNotice(feedOffline, lang),
			Reason:  feedOffline,
		}, nil
	}
	if partner.CanTake(settings.maxConcurrent) != nil {
		return FeedView{
			Partner: partnerView(partner, lang),
			Notice:  feedNotice(feedAtCapacity, lang),
			Reason:  feedAtCapacity,
		}, nil
	}

	nearby, err := uc.repo.WaitingNear(ctx, domain.Place{Lat: partner.Lat, Lng: partner.Lng},
		settings.partnerRadius, feedFanOut)
	if err != nil {
		return FeedView{}, storageError(err)
	}

	entries := make([]domain.FeedEntry, 0, len(nearby))
	for _, n := range nearby {
		entries = append(entries, domain.FeedEntry{Job: n.Job, DistanceToPickupM: n.DistanceM})
	}
	ordered := domain.BuildFeed(partner, entries)

	view := FeedView{Partner: partnerView(partner, lang), RadiusM: settings.partnerRadius}
	for _, entry := range ordered {
		view.Jobs = append(view.Jobs, jobView(entry.Job, entry.DistanceToPickupM, lang))
	}
	if len(view.Jobs) == 0 {
		view.Reason = feedNothingNearby
		view.Notice = feedNotice(feedNothingNearby, lang)
	}
	return view, nil
}

// MyJobs is the partner's own work — what they are holding now, or everything
// they have done.
func (uc *PartnerUseCase) MyJobs(ctx context.Context, userID string, liveOnly bool, limit, offset int, lang string) (JobListView, error) {
	partner, err := uc.mine(ctx, userID)
	if err != nil {
		return JobListView{}, err
	}
	jobs, total, err := uc.repo.Jobs(ctx, ports.JobFilter{
		PartnerID: partner.ID, LiveOnly: liveOnly,
		Limit: pageSize(limit), Offset: offset,
	})
	if err != nil {
		return JobListView{}, storageError(err)
	}
	out := JobListView{Total: total}
	for _, job := range jobs {
		out.Jobs = append(out.Jobs, jobView(job, 0, lang))
	}
	return out, nil
}

// Accept is a partner taking the job they were offered.
//
// The compare-and-set in SaveJob is what makes "two partners cannot be assigned
// the same order" true: the domain refuses a partner moving a job they do not
// hold, and the repository refuses the write when the job has moved since it
// was read.
func (uc *PartnerUseCase) Accept(ctx context.Context, userID, jobID, lang string) (JobView, error) {
	return uc.act(ctx, userID, jobID, lang, func(job *domain.Job, partner domain.Partner) error {
		return job.Accept(partner.ID, uc.clock.Now())
	}, afterAccept)
}

// Decline is a partner passing. The job goes back to waiting.
func (uc *PartnerUseCase) Decline(ctx context.Context, userID, jobID, lang string) (JobView, error) {
	return uc.act(ctx, userID, jobID, lang, func(job *domain.Job, partner domain.Partner) error {
		return job.Decline(partner.ID, uc.clock.Now())
	}, afterDecline)
}

// Collect is the rider having the goods, which moves the order too.
func (uc *PartnerUseCase) Collect(ctx context.Context, userID, jobID, lang string) (JobView, error) {
	return uc.act(ctx, userID, jobID, lang, func(job *domain.Job, partner domain.Partner) error {
		return job.Collect(partner.ID, uc.clock.Now())
	}, afterCollect)
}

// Deliver is the rider having handed them over.
func (uc *PartnerUseCase) Deliver(ctx context.Context, userID, jobID, lang string) (JobView, error) {
	return uc.act(ctx, userID, jobID, lang, func(job *domain.Job, partner domain.Partner) error {
		return job.Deliver(partner.ID, uc.clock.Now())
	}, afterDeliver)
}

// Fail is a delivery that could not be completed.
func (uc *PartnerUseCase) Fail(ctx context.Context, userID, jobID, reason, lang string) (JobView, error) {
	return uc.act(ctx, userID, jobID, lang, func(job *domain.Job, partner domain.Partner) error {
		return job.Fail(partner.ID, reason, uc.clock.Now())
	}, afterFail)
}

// aftermath is what happens to the partner and the order once a job has moved.
type aftermath int

const (
	afterAccept aftermath = iota
	afterDecline
	afterCollect
	afterDeliver
	afterFail
)

// act is the shared body of every partner action on a job: load, check, move,
// save, then settle up with the partner's load and the order's status.
func (uc *PartnerUseCase) act(
	ctx context.Context,
	userID, jobID, lang string,
	move func(*domain.Job, domain.Partner) error,
	after aftermath,
) (JobView, error) {
	partner, err := uc.mine(ctx, userID)
	if err != nil {
		return JobView{}, err
	}
	job, err := uc.repo.Job(ctx, jobID)
	if err != nil {
		return JobView{}, notFoundOr(err, "job")
	}

	expected := job.Status
	if err := move(&job, partner); err != nil {
		return JobView{}, jobError(err)
	}
	if err := uc.repo.SaveJob(ctx, job, expected); err != nil {
		return JobView{}, storageError(err)
	}

	uc.settle(ctx, job, partner, after)
	return jobView(job, 0, lang), nil
}

// settle adjusts the partner's load and moves the order along.
//
// Deliberately best-effort: the job has already been written, and a rider who
// tapped "delivered" must not be told it failed because a counter could not be
// incremented. What must not be best-effort is the order's status, so that one
// is reported — a delivered job whose order still says "on the way" is a
// customer watching a lie.
func (uc *PartnerUseCase) settle(ctx context.Context, job domain.Job, partner domain.Partner, after aftermath) {
	switch after {
	case afterAccept:
		partner.Carrying++
		_ = uc.repo.RecordOffer(ctx, partner.ID, true)
	case afterDecline:
		_ = uc.repo.RecordOffer(ctx, partner.ID, false)
	case afterDeliver, afterFail:
		if partner.Carrying > 0 {
			partner.Carrying--
		}
	case afterCollect:
	}

	// Busy is what being at the limit is called, not something a partner
	// declares — so it is maintained here rather than by the person.
	if partner.Availability != domain.AvailabilityOffline {
		partner.Availability = domain.AvailabilityAvailable
	}
	_ = uc.repo.SavePartner(ctx, partner)

	switch after {
	case afterCollect:
		_ = uc.order.Advance(ctx, job.OrderID, "picked_up", "partner", partner.ID, "")
	case afterDeliver:
		_ = uc.order.Advance(ctx, job.OrderID, "delivered", "partner", partner.ID, "")
	case afterFail:
		// Only a rider who was carrying the food fails the order. One who gave
		// the job up before collecting it has put it back on the board, and the
		// customer's order is still on its way.
		if job.Status == domain.JobFailed {
			_ = uc.order.Advance(ctx, job.OrderID, "failed", "partner", partner.ID, job.Reason)
		}
	case afterAccept, afterDecline:
	}
}

// rulesAt reads the dispatch configuration where a partner is standing.
//
// The partner's own location rather than a job's: the radius that decides what
// they are shown is a property of where they are working, and a rider in a
// rural upazila needs a wider one than a rider in Gulshan.
//
// A partner who has never reported a location resolves to nowhere, and gets the
// global defaults. That is the right answer rather than an error: they have no
// feed to compute either, and refusing to tell them their own settings would be
// refusing them the screen that explains why.
func (uc *PartnerUseCase) rulesAt(ctx context.Context, partner domain.Partner) (rules, error) {
	placement := cfg.Placement{}
	if partner.Lat != 0 || partner.Lng != 0 {
		area, err := uc.geo.ResolveDivision(ctx, geo.Point{Lat: partner.Lat, Lng: partner.Lng})
		if err == nil {
			placement = cfg.Placement{
				AreaCode:     area.AreaCode,
				DistrictCode: area.DistrictCode,
				DivisionCode: area.DivisionCode,
			}
		}
	}
	return resolveRules(ctx, uc.config, placement)
}

// mine loads the caller's partner record.
func (uc *PartnerUseCase) mine(ctx context.Context, userID string) (domain.Partner, error) {
	partner, found, err := uc.repo.PartnerOfUser(ctx, userID)
	if err != nil {
		return domain.Partner{}, storageError(err)
	}
	if !found {
		return domain.Partner{}, notFound("partner")
	}
	return partner, nil
}

// notFoundOr turns a missing row into a not-found and anything else into an
// outage.
func notFoundOr(err error, what string) error {
	if errs.Is(err, errs.KindNotFound) {
		return notFound(what)
	}
	return storageError(err)
}

// pageSize clamps a requested page rather than refusing one.
func pageSize(limit int) int {
	switch {
	case limit <= 0:
		return 20
	case limit > 50:
		return 50
	default:
		return limit
	}
}
