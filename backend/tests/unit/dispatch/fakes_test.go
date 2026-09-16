package dispatch

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"time"

	cfgcontract "github.com/rootlogic-lab/delivery/backend/internal/modules/config/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/dispatch/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/dispatch/domain"
	geocontract "github.com/rootlogic-lab/delivery/backend/internal/modules/geo/contract"
	ordercontract "github.com/rootlogic-lab/delivery/backend/internal/modules/order/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

var errBoom = errors.New("boom")

// fakeRepo is an in-memory dispatch store with the same guards the real one
// has: one partner per account, one job per order, and a compare-and-set on
// every job write.
type fakeRepo struct {
	partners map[string]domain.Partner
	jobs     map[string]domain.Job

	partnerErr  error
	savePartErr error
	createErr   error
	jobErr      error
	saveJobErr  error
	nearbyErr   error
	availErr    error
	listErr     error
	lapsedErr   error
	waitingErr  error

	// hideJobForOrderOnce makes the next JobForOrder miss, which is how the
	// loser of a race sees the world: it checks, finds nothing, and writes.
	hideJobForOrderOnce bool
	offerErr            error

	// nearby is what AvailableWithin returns, scripted so a test can place
	// partners at distances without a PostGIS.
	nearby []ports.PartnerDistance
	// waiting is what WaitingNear returns, likewise.
	waiting []ports.JobDistance
	// board overrides what the sweeper's second pass sees, so a test can hand
	// it a row that changed between the query and the loop.
	board []domain.Job

	offersRecorded []recordedOffer
}

type recordedOffer struct {
	partnerID string
	accepted  bool
}

func newRepo() *fakeRepo {
	return &fakeRepo{
		partners: map[string]domain.Partner{},
		jobs:     map[string]domain.Job{},
	}
}

func (r *fakeRepo) Partner(_ context.Context, partnerID string) (domain.Partner, error) {
	if r.partnerErr != nil {
		return domain.Partner{}, r.partnerErr
	}
	p, ok := r.partners[partnerID]
	if !ok {
		return domain.Partner{}, errs.New(errs.KindNotFound, "partner_not_found", "No such partner.")
	}
	return p, nil
}

func (r *fakeRepo) PartnerOfUser(_ context.Context, userID string) (domain.Partner, bool, error) {
	if r.partnerErr != nil {
		return domain.Partner{}, false, r.partnerErr
	}
	for _, p := range r.partners {
		if p.UserID == userID {
			return p, true, nil
		}
	}
	return domain.Partner{}, false, nil
}

func (r *fakeRepo) SavePartner(_ context.Context, p domain.Partner) error {
	if r.savePartErr != nil {
		return r.savePartErr
	}
	// The counters are not written by SavePartner in the real repository
	// either — RecordOffer owns them — so the fake preserves them too, or a
	// test would pass here and fail against Postgres.
	if existing, ok := r.partners[p.ID]; ok {
		p.Offered, p.Accepted = existing.Offered, existing.Accepted
	}
	r.partners[p.ID] = p
	return nil
}

func (r *fakeRepo) RecordOffer(_ context.Context, partnerID string, accepted bool) error {
	if r.offerErr != nil {
		return r.offerErr
	}
	r.offersRecorded = append(r.offersRecorded, recordedOffer{partnerID, accepted})
	p, ok := r.partners[partnerID]
	if !ok {
		return nil
	}
	if accepted {
		p.Accepted++
	} else {
		p.Offered++
	}
	r.partners[partnerID] = p
	return nil
}

func (r *fakeRepo) AvailableWithin(_ context.Context, _ domain.Place, _ float64, band domain.Band, limit int) ([]ports.PartnerDistance, error) {
	if r.availErr != nil {
		return nil, r.availErr
	}
	out := make([]ports.PartnerDistance, 0, len(r.nearby))
	for _, candidate := range r.nearby {
		// The real query applies D4 in its own WHERE clause, so the fake does
		// too — a test that passed here and failed against Postgres would be
		// testing the fake.
		if !candidate.Partner.Preference.Accepts(band) {
			continue
		}
		if candidate.Partner.Availability != domain.AvailabilityAvailable {
			continue
		}
		out = append(out, candidate)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DistanceM < out[j].DistanceM })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *fakeRepo) CreateJob(_ context.Context, j domain.Job) error {
	if r.createErr != nil {
		return r.createErr
	}
	for _, existing := range r.jobs {
		if existing.OrderID == j.OrderID {
			return errors.New("duplicate order")
		}
	}
	r.jobs[j.ID] = j
	return nil
}

func (r *fakeRepo) Job(_ context.Context, jobID string) (domain.Job, error) {
	if r.jobErr != nil {
		return domain.Job{}, r.jobErr
	}
	j, ok := r.jobs[jobID]
	if !ok {
		return domain.Job{}, errs.New(errs.KindNotFound, "job_not_found", "No such delivery.")
	}
	return j, nil
}

func (r *fakeRepo) JobForOrder(_ context.Context, orderID string) (domain.Job, bool, error) {
	if r.hideJobForOrderOnce {
		r.hideJobForOrderOnce = false
		return domain.Job{}, false, nil
	}
	if r.jobErr != nil {
		return domain.Job{}, false, r.jobErr
	}
	for _, j := range r.jobs {
		if j.OrderID == orderID {
			return j, true, nil
		}
	}
	return domain.Job{}, false, nil
}

func (r *fakeRepo) Jobs(_ context.Context, f ports.JobFilter) ([]domain.Job, int, error) {
	if r.listErr != nil {
		return nil, 0, r.listErr
	}
	var out []domain.Job
	for _, j := range r.jobs {
		if f.PartnerID == "" || j.PartnerID != f.PartnerID {
			continue
		}
		if f.LiveOnly && !j.Status.IsLive() {
			continue
		}
		out = append(out, j)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	total := len(out)
	if f.Offset >= len(out) {
		return nil, total, nil
	}
	out = out[f.Offset:]
	if f.Limit > 0 && f.Limit < len(out) {
		out = out[:f.Limit]
	}
	return out, total, nil
}

// SaveJob carries the same compare-and-set the real one does. It is the half of
// "two partners cannot be assigned the same order" that lives in storage.
func (r *fakeRepo) SaveJob(_ context.Context, j domain.Job, expected domain.JobStatus) error {
	if r.saveJobErr != nil {
		return r.saveJobErr
	}
	current, ok := r.jobs[j.ID]
	if !ok {
		return errs.New(errs.KindNotFound, "job_not_found", "No such delivery.")
	}
	if current.Status != expected {
		return errs.New(errs.KindConflict, "job_moved", "Somebody else has taken that delivery.")
	}
	r.jobs[j.ID] = j
	return nil
}

func (r *fakeRepo) WaitingNear(_ context.Context, _ domain.Place, _ float64, limit int) ([]ports.JobDistance, error) {
	if r.nearbyErr != nil {
		return nil, r.nearbyErr
	}
	out := make([]ports.JobDistance, 0, len(r.waiting))
	for _, entry := range r.waiting {
		// Reflect whatever has happened to the job since the script was set,
		// so a job somebody accepted stops appearing.
		if current, ok := r.jobs[entry.Job.ID]; ok {
			entry.Job = current
		}
		if entry.Job.Status != domain.JobWaiting {
			continue
		}
		out = append(out, entry)
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *fakeRepo) WaitingJobs(_ context.Context, limit int) ([]domain.Job, error) {
	if r.waitingErr != nil {
		return nil, r.waitingErr
	}
	if r.board != nil {
		return r.board, nil
	}
	var out []domain.Job
	for _, j := range r.jobs {
		if j.Status == domain.JobWaiting {
			out = append(out, j)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *fakeRepo) LapsedOffers(_ context.Context, limit int) ([]domain.Job, error) {
	if r.lapsedErr != nil {
		return nil, r.lapsedErr
	}
	var out []domain.Job
	for _, j := range r.jobs {
		if j.Status == domain.JobOffered {
			out = append(out, j)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// fakeOrder stands in for the order module.
type fakeOrder struct {
	order      ordercontract.Order
	orderErr   error
	advanceErr error
	advanced   []advanceCall
}

type advanceCall struct {
	orderID, to, actor, actorID, reason string
}

func (o *fakeOrder) Order(context.Context, string) (ordercontract.Order, error) {
	return o.order, o.orderErr
}

func (o *fakeOrder) Advance(_ context.Context, orderID, to, actor, actorID, reason string) error {
	o.advanced = append(o.advanced, advanceCall{orderID, to, actor, actorID, reason})
	return o.advanceErr
}

// fakeGeo stands in for the geo module.
type fakeGeo struct {
	area     geocontract.Area
	areaErr  error
	distance float64
	distErr  error
}

func (g *fakeGeo) DistanceBetween(context.Context, geocontract.Point, geocontract.Point) (float64, error) {
	return g.distance, g.distErr
}

func (g *fakeGeo) ResolveDivision(context.Context, geocontract.Point) (geocontract.Area, error) {
	return g.area, g.areaErr
}

// fakeSettings and fakeConfig stand in for the config module.
type fakeSettings struct {
	ints   map[string]int64
	failOn string
}

func (s fakeSettings) Int(key string) (int64, error) {
	if key == s.failOn {
		return 0, errBoom
	}
	v, ok := s.ints[key]
	if !ok {
		return 0, errBoom
	}
	return v, nil
}

func (s fakeSettings) Bool(string) (bool, error)     { return false, errBoom }
func (s fakeSettings) Ratio(string) (float64, error) { return 0, errBoom }

type fakeConfig struct {
	settings cfgcontract.Settings
	err      error
	seen     []cfgcontract.Placement
}

func (f *fakeConfig) Settings(_ context.Context, p cfgcontract.Placement) (cfgcontract.Settings, error) {
	f.seen = append(f.seen, p)
	if f.err != nil {
		return nil, f.err
	}
	return f.settings, nil
}

// appendixB is the default dispatch configuration: 5 km short, 25 km long,
// a 7 km partner radius, a 30-second offer and three concurrent jobs.
func appendixB() fakeSettings {
	return fakeSettings{ints: map[string]int64{
		cfgcontract.DispatchShortDistance: 5000,
		cfgcontract.DispatchLongDistance:  25000,
		cfgcontract.DispatchPartnerRadius: 7000,
		cfgcontract.DispatchAssignTimeout: 30,
		cfgcontract.DispatchMaxConcurrent: 3,
	}}
}

// fakeIDs hands out predictable ids.
type fakeIDs struct{ n int }

func (g *fakeIDs) New(prefix string) string {
	g.n++
	return prefix + "-" + strconv.Itoa(g.n)
}

// movableClock is a clock a test can wind forward.
type movableClock struct{ now time.Time }

func (c *movableClock) Now() time.Time { return c.now }
