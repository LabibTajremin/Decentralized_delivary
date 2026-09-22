// Package postgres stores delivery partners and jobs.
//
// Engine-specific SQL is confined to this package (05-architecture.md 2.4).
// This is the second module after geo to depend on PostGIS rather than merely
// store coordinates: the candidate pool for an assignment round and the jobs in
// a partner's feed are both spatial queries, and doing either in Go would mean
// pulling a city's worth of rows into memory.
package postgres

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/dispatch/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/dispatch/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// uniqueViolation is Postgres' SQLSTATE for a broken unique constraint. The one
// that matters here is delivery_jobs_order_idx: two "the order is ready"
// events for the same order, racing.
const uniqueViolation = "23505"

// Querier is the read and write surface this repository needs.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Repository stores partners and jobs.
type Repository struct {
	db Querier
}

// New builds a repository.
func New(db Querier) *Repository { return &Repository{db: db} }

// NewFromPool is the production constructor.
func NewFromPool(pool *pgxpool.Pool) *Repository { return &Repository{db: pool} }

// ------------------------------------------------------------------ partners

// partnerColumns is the projection every partner read uses, in one place so
// the query and the scan cannot drift apart.
//
// The pin comes back as two numbers rather than a geography, because nothing
// above this package knows what a geography is.
const partnerColumns = `
	id, user_id, name, phone, vehicle, availability, preference,
	COALESCE(ST_Y(pin::geometry), 0), COALESCE(ST_X(pin::geometry), 0),
	carrying, offered, accepted`

func scanPartner(row pgx.Row) (domain.Partner, error) {
	var p domain.Partner
	var availability, preference string
	err := row.Scan(&p.ID, &p.UserID, &p.Name, &p.Phone, &p.Vehicle,
		&availability, &preference, &p.Lat, &p.Lng,
		&p.Carrying, &p.Offered, &p.Accepted)
	if err != nil {
		return domain.Partner{}, err
	}
	p.Availability = domain.Availability(availability)
	p.Preference = domain.Preference(preference)
	return p, nil
}

// Partner returns one partner by their own id.
func (r *Repository) Partner(ctx context.Context, partnerID string) (domain.Partner, error) {
	partner, err := scanPartner(r.db.QueryRow(ctx,
		`SELECT`+partnerColumns+` FROM delivery_partners WHERE id = $1`, partnerID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Partner{}, errs.New(errs.KindNotFound, "partner_not_found",
			"We could not find that delivery partner.")
	}
	return partner, err
}

// PartnerOfUser returns the partner record for an account.
func (r *Repository) PartnerOfUser(ctx context.Context, userID string) (domain.Partner, bool, error) {
	partner, err := scanPartner(r.db.QueryRow(ctx,
		`SELECT`+partnerColumns+` FROM delivery_partners WHERE user_id = $1`, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Partner{}, false, nil
	}
	if err != nil {
		return domain.Partner{}, false, err
	}
	return partner, true, nil
}

// SavePartner writes a partner, creating them if absent.
//
// The counters are deliberately not written here. They are incremented by
// RecordOffer, and including them in this upsert would let a stale read
// overwrite offers made between the read and the write.
func (r *Repository) SavePartner(ctx context.Context, p domain.Partner) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO delivery_partners (
			id, user_id, name, phone, vehicle, availability, preference,
			pin, carrying, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,
			ST_SetSRID(ST_MakePoint($8, $9), 4326)::geography, $10, now())
		ON CONFLICT (id) DO UPDATE SET
			name         = EXCLUDED.name,
			phone        = EXCLUDED.phone,
			vehicle      = EXCLUDED.vehicle,
			availability = EXCLUDED.availability,
			preference   = EXCLUDED.preference,
			pin          = EXCLUDED.pin,
			carrying     = EXCLUDED.carrying,
			updated_at   = now()`,
		p.ID, p.UserID, p.Name, p.Phone, p.Vehicle,
		string(p.Availability), string(p.Preference),
		p.Lng, p.Lat, p.Carrying)
	return err
}

// RecordOffer increments the offer counters.
//
// Two increments in one statement rather than a read-modify-write: a partner
// being offered two jobs at once would otherwise lose one of the counts, and
// the acceptance rate is what decides who gets offered work next.
func (r *Repository) RecordOffer(ctx context.Context, partnerID string, accepted bool) error {
	accepts := 0
	if accepted {
		accepts = 1
	}
	_, err := r.db.Exec(ctx, `
		UPDATE delivery_partners
		SET offered = offered + CASE WHEN $2 THEN 0 ELSE 1 END,
		    accepted = accepted + $3,
		    updated_at = now()
		WHERE id = $1`, partnerID, accepted, accepts)
	return err
}

// AvailableWithin returns the candidate pool for one assignment round (ALG-04).
//
// ST_DWithin with the GiST index, use_spheroid false — the same choice geo
// makes, for the same reason: a sphere is accurate to a few metres over a
// city and materially faster.
func (r *Repository) AvailableWithin(
	ctx context.Context, at domain.Place, radiusM float64, band domain.Band, limit int,
) ([]ports.PartnerDistance, error) {
	rows, err := r.db.Query(ctx, `
		SELECT`+partnerColumns+`,
			ST_Distance(pin, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, false)
		FROM delivery_partners
		WHERE availability = 'available'
		  AND pin IS NOT NULL
		  AND ST_DWithin(pin, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, $3, false)
		  -- D4, in the query rather than a filter afterwards: a partner who
		  -- only wants local work is not a candidate for a long job at all.
		  AND (preference = 'any' OR preference = $4 OR $4 = 'beyond')
		ORDER BY 13
		LIMIT $5`,
		at.Lng, at.Lat, radiusM, string(band), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ports.PartnerDistance
	for rows.Next() {
		var p domain.Partner
		var availability, preference string
		var distance float64
		if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.Phone, &p.Vehicle,
			&availability, &preference, &p.Lat, &p.Lng,
			&p.Carrying, &p.Offered, &p.Accepted, &distance); err != nil {
			return nil, err
		}
		p.Availability = domain.Availability(availability)
		p.Preference = domain.Preference(preference)
		out = append(out, ports.PartnerDistance{Partner: p, DistanceM: distance})
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------- jobs

const jobColumns = `
	id, order_id, code, status, COALESCE(partner_id, ''), passed_by,
	pickup_name, pickup_phone, pickup_single,
	COALESCE(ST_Y(pickup_pin::geometry), 0), COALESCE(ST_X(pickup_pin::geometry), 0),
	destination_name, destination_phone, destination_single,
	COALESCE(ST_Y(destination_pin::geometry), 0), COALESCE(ST_X(destination_pin::geometry), 0),
	distance_m, band, area_code, district_code, division_code, reason, attempts,
	COALESCE(offered_at, 'epoch'::timestamptz), COALESCE(offer_expires_at, 'epoch'::timestamptz),
	created_at, updated_at`

// jobRow is the mutable half of a scan: the columns whose Go types are not the
// domain's own. Kept in one place so the two scanners below cannot drift.
type jobRow struct {
	status    string
	band      string
	offeredAt time.Time
	expiresAt time.Time
}

// dests lists where jobColumns lands, in order.
func (raw *jobRow) dests(j *domain.Job) []any {
	return []any{
		&j.ID, &j.OrderID, &j.Code, &raw.status, &j.PartnerID, &j.PassedBy,
		&j.Pickup.Name, &j.Pickup.Phone, &j.Pickup.SingleLine, &j.Pickup.Lat, &j.Pickup.Lng,
		&j.Destination.Name, &j.Destination.Phone, &j.Destination.SingleLine,
		&j.Destination.Lat, &j.Destination.Lng,
		&j.DistanceM, &raw.band,
		&j.Placement.AreaCode, &j.Placement.DistrictCode, &j.Placement.DivisionCode,
		&j.Reason, &j.Attempts,
		&raw.offeredAt, &raw.expiresAt, &j.CreatedAt, &j.UpdatedAt,
	}
}

// settle puts the scanned raw columns back onto the job.
func (raw *jobRow) settle(j *domain.Job) {
	j.Status = domain.JobStatus(raw.status)
	j.Band = domain.Band(raw.band)
	// An epoch timestamp is how a NULL comes back through the COALESCE above.
	// Restored to the zero time here so nothing upstream has to know that.
	if raw.offeredAt.Unix() != 0 {
		j.OfferedAt = raw.offeredAt
	}
	if raw.expiresAt.Unix() != 0 {
		j.OfferExpiresAt = raw.expiresAt
	}
}

func scanJob(row pgx.Row) (domain.Job, error) {
	var j domain.Job
	var raw jobRow
	if err := row.Scan(raw.dests(&j)...); err != nil {
		return domain.Job{}, err
	}
	raw.settle(&j)
	return j, nil
}

// scanJobWithDistance reads jobColumns plus a trailing distance.
func scanJobWithDistance(rows pgx.Rows) (domain.Job, float64, error) {
	var j domain.Job
	var raw jobRow
	var distance float64
	if err := rows.Scan(append(raw.dests(&j), &distance)...); err != nil {
		return domain.Job{}, 0, err
	}
	raw.settle(&j)
	return j, distance, nil
}

// CreateJob writes a new job.
func (r *Repository) CreateJob(ctx context.Context, j domain.Job) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO delivery_jobs (
			id, order_id, code, status, partner_id, passed_by,
			pickup_name, pickup_phone, pickup_single, pickup_pin,
			destination_name, destination_phone, destination_single, destination_pin,
			distance_m, band, area_code, district_code, division_code,
			reason, attempts, offered_at, offer_expires_at,
			created_at, updated_at)
		VALUES ($1,$2,$3,$4,NULL,'',
			$5,$6,$7, ST_SetSRID(ST_MakePoint($8, $9), 4326)::geography,
			$10,$11,$12, ST_SetSRID(ST_MakePoint($13, $14), 4326)::geography,
			$15,$16,$17,$18,$19,'',0,NULL,NULL,$20,$21)`,
		j.ID, j.OrderID, j.Code, string(j.Status),
		j.Pickup.Name, j.Pickup.Phone, j.Pickup.SingleLine, j.Pickup.Lng, j.Pickup.Lat,
		j.Destination.Name, j.Destination.Phone, j.Destination.SingleLine,
		j.Destination.Lng, j.Destination.Lat,
		j.DistanceM, string(j.Band),
		j.Placement.AreaCode, j.Placement.DistrictCode, j.Placement.DivisionCode,
		j.CreatedAt, j.UpdatedAt)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
		// The order already has a job. Reported as a conflict rather than an
		// outage so the caller can go and read the job that won.
		return errs.New(errs.KindConflict, "job_exists",
			"That order is already out for delivery.")
	}
	return err
}

// Job returns one delivery.
func (r *Repository) Job(ctx context.Context, jobID string) (domain.Job, error) {
	job, err := scanJob(r.db.QueryRow(ctx,
		`SELECT`+jobColumns+` FROM delivery_jobs WHERE id = $1`, jobID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Job{}, errs.New(errs.KindNotFound, "job_not_found",
			"We could not find that delivery.")
	}
	return job, err
}

// JobForOrder returns the delivery for an order, if there is one.
func (r *Repository) JobForOrder(ctx context.Context, orderID string) (domain.Job, bool, error) {
	job, err := scanJob(r.db.QueryRow(ctx,
		`SELECT`+jobColumns+` FROM delivery_jobs WHERE order_id = $1`, orderID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Job{}, false, nil
	}
	if err != nil {
		return domain.Job{}, false, err
	}
	return job, true, nil
}

// Jobs lists deliveries matching a filter, newest first.
func (r *Repository) Jobs(ctx context.Context, f ports.JobFilter) ([]domain.Job, int, error) {
	where, args := jobFilterClause(f)

	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM delivery_jobs`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	rows, err := r.db.Query(ctx,
		`SELECT`+jobColumns+` FROM delivery_jobs`+where+
			` ORDER BY created_at DESC, id DESC LIMIT $`+strconv.Itoa(len(args)+1)+
			` OFFSET $`+strconv.Itoa(len(args)+2),
		append(args, f.Limit, f.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var jobs []domain.Job
	for rows.Next() {
		job, scanErr := scanJob(rows)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		jobs = append(jobs, job)
	}
	return jobs, total, rows.Err()
}

// jobFilterClause builds the WHERE for a listing.
//
// A filter with no partner would return everybody's jobs, so it returns a
// predicate that matches nothing instead. That is a caller bug, and answering
// it with the whole table is how a caller bug becomes a data leak.
func jobFilterClause(f ports.JobFilter) (string, []any) {
	clauses := make([]string, 0, 3)
	args := make([]any, 0, 3)

	if f.PartnerID == "" {
		clauses = append(clauses, `FALSE`)
	} else {
		args = append(args, f.PartnerID)
		clauses = append(clauses, `partner_id = $`+strconv.Itoa(len(args)))
	}
	if f.LiveOnly {
		clauses = append(clauses, `status NOT IN ('delivered','failed','cancelled')`)
	}
	if len(f.Statuses) > 0 {
		names := make([]string, 0, len(f.Statuses))
		for _, s := range f.Statuses {
			args = append(args, string(s))
			names = append(names, `$`+strconv.Itoa(len(args)))
		}
		clauses = append(clauses, `status IN (`+strings.Join(names, ",")+`)`)
	}
	return ` WHERE ` + strings.Join(clauses, " AND "), args
}

// SaveJob writes a job, checking it is still where the caller thought it was.
//
// The expected status is in the UPDATE's own WHERE clause. This is the half of
// "two partners cannot be assigned the same order" that the domain cannot
// enforce: two riders accepting the same waiting job in the same millisecond
// both pass the domain check and only one of them updates a row.
func (r *Repository) SaveJob(ctx context.Context, j domain.Job, expected domain.JobStatus) error {
	partnerID := any(j.PartnerID)
	if j.PartnerID == "" {
		partnerID = nil
	}
	offeredAt := any(j.OfferedAt)
	if j.OfferedAt.IsZero() {
		offeredAt = nil
	}
	expiresAt := any(j.OfferExpiresAt)
	if j.OfferExpiresAt.IsZero() {
		expiresAt = nil
	}

	tag, err := r.db.Exec(ctx, `
		UPDATE delivery_jobs
		SET status = $1, partner_id = $2, passed_by = $3, reason = $4, attempts = $5,
		    offered_at = $6, offer_expires_at = $7, updated_at = $8
		WHERE id = $9 AND status = $10`,
		string(j.Status), partnerID, j.PassedBy, j.Reason, j.Attempts,
		offeredAt, expiresAt, j.UpdatedAt, j.ID, string(expected))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errs.New(errs.KindConflict, "job_moved",
			"Somebody else has taken that delivery.")
	}
	return nil
}

// WaitingNear returns the waiting jobs whose pickup is inside radiusM of a
// point — the raw material of a partner's feed (ALG-08).
func (r *Repository) WaitingNear(
	ctx context.Context, at domain.Place, radiusM float64, limit int,
) ([]ports.JobDistance, error) {
	rows, err := r.db.Query(ctx, `
		SELECT`+jobColumns+`,
			ST_Distance(pickup_pin, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, false)
				AS distance_to_partner_m
		FROM delivery_jobs
		WHERE status = 'waiting'
		  AND pickup_pin IS NOT NULL
		  AND ST_DWithin(pickup_pin, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, $3, false)
		ORDER BY distance_to_partner_m
		LIMIT $4`,
		at.Lng, at.Lat, radiusM, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ports.JobDistance
	for rows.Next() {
		job, distance, scanErr := scanJobWithDistance(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, ports.JobDistance{Job: job, DistanceM: distance})
	}
	return out, rows.Err()
}

// LapsedOffers returns jobs whose offer has run out.
func (r *Repository) LapsedOffers(ctx context.Context, limit int) ([]domain.Job, error) {
	rows, err := r.db.Query(ctx, `
		SELECT`+jobColumns+`
		FROM delivery_jobs
		WHERE status = 'offered' AND offer_expires_at IS NOT NULL AND offer_expires_at <= now()
		ORDER BY offer_expires_at
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Job
	for rows.Next() {
		job, scanErr := scanJob(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, job)
	}
	return out, rows.Err()
}

// WaitingJobs returns jobs nobody holds, oldest first.
func (r *Repository) WaitingJobs(ctx context.Context, limit int) ([]domain.Job, error) {
	rows, err := r.db.Query(ctx, `
		SELECT`+jobColumns+`
		FROM delivery_jobs
		WHERE status = 'waiting'
		ORDER BY created_at
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Job
	for rows.Next() {
		job, scanErr := scanJob(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, job)
	}
	return out, rows.Err()
}
