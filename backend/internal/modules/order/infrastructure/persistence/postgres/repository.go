// Package postgres stores orders.
//
// Engine-specific SQL is confined to this package (05-architecture.md 2.4).
package postgres

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/order/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/order/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/money"
)

// Querier is the read and write surface this repository needs.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// TxBeginner starts a transaction.
type TxBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Repository stores orders, their lines and their history.
type Repository struct {
	db Querier
	tx TxBeginner
}

// New builds a repository.
func New(db Querier, tx TxBeginner) *Repository { return &Repository{db: db, tx: tx} }

// NewFromPool is the production constructor.
func NewFromPool(pool *pgxpool.Pool) *Repository { return &Repository{db: pool, tx: pool} }

// orderColumns is the projection every read uses, in one place so the scan and
// the query cannot drift apart.
const orderColumns = `
	id, code, customer_id, merchant_id, status, payment_method,
	subtotal_minor, delivery_minor, expansion_surcharge_minor, total_minor,
	free_delivery, expanded, expansion_level, distance_m,
	address_id, address_label, recipient_name, recipient_phone,
	address_line1, address_line2, address_single, address_lat, address_lng,
	address_area, address_area_name, address_directions,
	pickup_name, pickup_phone, pickup_single, pickup_lat, pickup_lng,
	placed_at, updated_at`

// scanOrder reads one row of orderColumns.
func scanOrder(row pgx.Row) (domain.Order, error) {
	var o domain.Order
	var status, payment string
	var subtotal, delivery, surcharge, total int64

	err := row.Scan(
		&o.ID, &o.Code, &o.CustomerID, &o.MerchantID, &status, &payment,
		&subtotal, &delivery, &surcharge, &total,
		&o.Charges.FreeDelivery, &o.Charges.Expanded, &o.ExpansionLevel, &o.Charges.DistanceM,
		&o.Destination.AddressID, &o.Destination.Label, &o.Destination.Recipient, &o.Destination.Phone,
		&o.Destination.Line1, &o.Destination.Line2, &o.Destination.SingleLine,
		&o.Destination.Lat, &o.Destination.Lng,
		&o.Destination.AreaCode, &o.Destination.AreaName, &o.Destination.Directions,
		&o.Pickup.Name, &o.Pickup.Phone, &o.Pickup.SingleLine, &o.Pickup.Lat, &o.Pickup.Lng,
		&o.PlacedAt, &o.UpdatedAt,
	)
	if err != nil {
		return domain.Order{}, err
	}
	o.Status = domain.Status(status)
	o.Payment = domain.PaymentMethod(payment)
	o.Charges.Subtotal = money.Taka(subtotal)
	o.Charges.Delivery = money.Taka(delivery)
	o.Charges.ExpansionSurcharge = money.Taka(surcharge)
	o.Charges.Total = money.Taka(total)
	o.Pickup.MerchantID = o.MerchantID
	return o, nil
}

// Create writes a whole order in one transaction.
//
// Lines, their options and the first event go in together. An order half
// written is an order the customer paid for and the shop never saw.
func (r *Repository) Create(ctx context.Context, o domain.Order) error {
	tx, err := r.tx.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := insertOrder(ctx, tx, o); err != nil {
		return err
	}
	if err := insertLines(ctx, tx, o); err != nil {
		return err
	}
	for _, e := range o.Events {
		if err := insertEvent(ctx, tx, o.ID, e); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func insertOrder(ctx context.Context, tx pgx.Tx, o domain.Order) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO orders (
			id, code, customer_id, merchant_id, status, payment_method,
			subtotal_minor, delivery_minor, expansion_surcharge_minor, total_minor,
			free_delivery, expanded, expansion_level, distance_m,
			address_id, address_label, recipient_name, recipient_phone,
			address_line1, address_line2, address_single, address_lat, address_lng,
			address_area, address_area_name, address_directions,
			pickup_name, pickup_phone, pickup_single, pickup_lat, pickup_lng,
			placed_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,
			$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,$31,$32,$33)`,
		o.ID, o.Code, o.CustomerID, o.MerchantID, string(o.Status), string(o.Payment),
		o.Charges.Subtotal.Minor(), o.Charges.Delivery.Minor(),
		o.Charges.ExpansionSurcharge.Minor(), o.Charges.Total.Minor(),
		o.Charges.FreeDelivery, o.Charges.Expanded, o.ExpansionLevel, o.Charges.DistanceM,
		o.Destination.AddressID, o.Destination.Label, o.Destination.Recipient, o.Destination.Phone,
		o.Destination.Line1, o.Destination.Line2, o.Destination.SingleLine,
		o.Destination.Lat, o.Destination.Lng,
		o.Destination.AreaCode, o.Destination.AreaName, o.Destination.Directions,
		o.Pickup.Name, o.Pickup.Phone, o.Pickup.SingleLine, o.Pickup.Lat, o.Pickup.Lng,
		o.PlacedAt, o.UpdatedAt)
	return err
}

func insertLines(ctx context.Context, tx pgx.Tx, o domain.Order) error {
	for position, line := range o.Lines {
		if _, err := tx.Exec(ctx, `
			INSERT INTO order_lines (id, order_id, kind, target_id, name,
				unit_price_minor, quantity, note, position)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			line.ID, o.ID, line.Kind, line.TargetID, line.Name,
			line.UnitPrice.Minor(), line.Quantity, line.Note, position); err != nil {
			return err
		}
		for optionPosition, option := range line.Options {
			if _, err := tx.Exec(ctx, `
				INSERT INTO order_line_options (line_id, group_id, option_id, name, price_minor, position)
				VALUES ($1,$2,$3,$4,$5,$6)`,
				line.ID, option.GroupID, option.OptionID, option.Name,
				option.Price.Minor(), optionPosition); err != nil {
				return err
			}
		}
	}
	return nil
}

func insertEvent(ctx context.Context, q Querier, orderID string, e domain.Event) error {
	_, err := q.Exec(ctx, `
		INSERT INTO order_events (id, order_id, status, actor, actor_id, reason, at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		e.ID, orderID, string(e.Status), string(e.Actor), e.ActorID, e.Reason, e.At)
	return err
}

// Order returns one order with its lines and history.
func (r *Repository) Order(ctx context.Context, orderID string) (domain.Order, error) {
	order, err := scanOrder(r.db.QueryRow(ctx,
		`SELECT`+orderColumns+` FROM orders WHERE id = $1`, orderID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Order{}, errs.New(errs.KindNotFound, "order_not_found", "We could not find that order.")
	}
	if err != nil {
		return domain.Order{}, err
	}

	if order.Lines, err = r.linesOf(ctx, []string{orderID}); err != nil {
		return domain.Order{}, err
	}
	if order.Events, err = r.eventsOf(ctx, []string{orderID}); err != nil {
		return domain.Order{}, err
	}
	return order, nil
}

// Orders lists orders matching a filter, newest first.
//
// Three queries for a page rather than one per order: the rows, then every
// line, then every event, joined up in Go. A shop opening its queue in the
// morning should not make sixty round trips.
func (r *Repository) Orders(ctx context.Context, f ports.Filter) ([]domain.Order, int, error) {
	where, args := filterClause(f)

	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM orders`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	rows, err := r.db.Query(ctx,
		`SELECT`+orderColumns+` FROM orders`+where+
			` ORDER BY placed_at DESC, id DESC LIMIT $`+strconv.Itoa(len(args)+1)+
			` OFFSET $`+strconv.Itoa(len(args)+2),
		append(args, f.Limit, f.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var orders []domain.Order
	var ids []string
	for rows.Next() {
		order, scanErr := scanOrder(rows)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		orders = append(orders, order)
		ids = append(ids, order.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if len(orders) == 0 {
		return nil, total, nil
	}

	lines, err := r.linesByOrder(ctx, ids)
	if err != nil {
		return nil, 0, err
	}
	events, err := r.eventsByOrder(ctx, ids)
	if err != nil {
		return nil, 0, err
	}
	for i := range orders {
		orders[i].Lines = lines[orders[i].ID]
		orders[i].Events = events[orders[i].ID]
	}
	return orders, total, nil
}

// filterClause builds the WHERE for a listing.
//
// A filter with neither a customer nor a merchant would return everybody's
// orders, so it returns a predicate that matches nothing instead. That is a
// caller bug, and answering it with the whole table is how a caller bug becomes
// a data leak.
func filterClause(f ports.Filter) (string, []any) {
	clauses := make([]string, 0, 4)
	args := make([]any, 0, 4)

	switch {
	case f.CustomerID != "":
		args = append(args, f.CustomerID)
		clauses = append(clauses, `customer_id = $`+strconv.Itoa(len(args)))
	case f.MerchantID != "":
		args = append(args, f.MerchantID)
		clauses = append(clauses, `merchant_id = $`+strconv.Itoa(len(args)))
	default:
		clauses = append(clauses, `FALSE`)
	}

	if f.LiveOnly {
		clauses = append(clauses, `status NOT IN ('delivered','cancelled','rejected','failed')`)
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

// linesOf returns the lines of one order, in order.
func (r *Repository) linesOf(ctx context.Context, orderIDs []string) ([]domain.Line, error) {
	byOrder, err := r.linesByOrder(ctx, orderIDs)
	if err != nil {
		return nil, err
	}
	return byOrder[orderIDs[0]], nil
}

// linesByOrder reads lines and their options for several orders at once.
func (r *Repository) linesByOrder(ctx context.Context, orderIDs []string) (map[string][]domain.Line, error) {
	rows, err := r.db.Query(ctx, `
		SELECT order_id, id, kind, target_id, name, unit_price_minor, quantity, note
		FROM order_lines WHERE order_id = ANY($1) ORDER BY order_id, position, id`, orderIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// The lines are collected flat and grouped at the end. Holding pointers
	// into a map of slices does not work: append reallocates, and every
	// pointer taken before it silently stops referring to the row it was taken
	// from — which shows up as options quietly vanishing from an order.
	type placedLine struct {
		orderID string
		line    domain.Line
	}
	var lines []placedLine
	at := map[string]int{}
	for rows.Next() {
		var entry placedLine
		var minor int64
		if err := rows.Scan(&entry.orderID, &entry.line.ID, &entry.line.Kind,
			&entry.line.TargetID, &entry.line.Name, &minor,
			&entry.line.Quantity, &entry.line.Note); err != nil {
			return nil, err
		}
		entry.line.UnitPrice = money.Taka(minor)
		at[entry.line.ID] = len(lines)
		lines = append(lines, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	byOrder := map[string][]domain.Line{}
	if len(lines) == 0 {
		return byOrder, nil
	}

	options, err := r.db.Query(ctx, `
		SELECT o.line_id, o.group_id, o.option_id, o.name, o.price_minor
		FROM order_line_options o
		JOIN order_lines l ON l.id = o.line_id
		WHERE l.order_id = ANY($1)
		ORDER BY o.position, o.group_id, o.option_id`, orderIDs)
	if err != nil {
		return nil, err
	}
	defer options.Close()

	for options.Next() {
		var lineID string
		var option domain.Option
		var minor int64
		if err := options.Scan(&lineID, &option.GroupID, &option.OptionID,
			&option.Name, &minor); err != nil {
			return nil, err
		}
		option.Price = money.Taka(minor)
		index, ok := at[lineID]
		if !ok {
			// An option whose line is not in the set just read. Impossible
			// through the join above; skipped rather than indexed blindly,
			// because a panic here would be a 500 on an order screen.
			continue
		}
		lines[index].line.Options = append(lines[index].line.Options, option)
	}
	if err := options.Err(); err != nil {
		return nil, err
	}

	for _, entry := range lines {
		byOrder[entry.orderID] = append(byOrder[entry.orderID], entry.line)
	}
	return byOrder, nil
}

// eventsOf returns the history of one order.
func (r *Repository) eventsOf(ctx context.Context, orderIDs []string) ([]domain.Event, error) {
	byOrder, err := r.eventsByOrder(ctx, orderIDs)
	if err != nil {
		return nil, err
	}
	return byOrder[orderIDs[0]], nil
}

// eventsByOrder reads the history of several orders at once.
func (r *Repository) eventsByOrder(ctx context.Context, orderIDs []string) (map[string][]domain.Event, error) {
	rows, err := r.db.Query(ctx, `
		SELECT order_id, id, status, actor, actor_id, reason, at
		FROM order_events WHERE order_id = ANY($1) ORDER BY order_id, at, id`, orderIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byOrder := map[string][]domain.Event{}
	for rows.Next() {
		var orderID, status, actor string
		var e domain.Event
		if err := rows.Scan(&orderID, &e.ID, &status, &actor, &e.ActorID, &e.Reason, &e.At); err != nil {
			return nil, err
		}
		e.Status = domain.Status(status)
		e.Actor = domain.Actor(actor)
		byOrder[orderID] = append(byOrder[orderID], e)
	}
	return byOrder, rows.Err()
}

// AppendTransition writes a status change and its event atomically.
//
// The expected status is in the UPDATE's own WHERE clause, so two riders
// tapping "picked up" at the same moment cannot both succeed: the second
// updates no rows and is told the order moved under it. The state machine says
// what may happen; this says it happened once.
func (r *Repository) AppendTransition(ctx context.Context, orderID string, from domain.Status, e domain.Event) error {
	tx, err := r.tx.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `
		UPDATE orders SET status = $1, updated_at = $2
		WHERE id = $3 AND status = $4`,
		string(e.Status), e.At, orderID, string(from))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errs.New(errs.KindConflict, "order_moved",
			"That order has already changed. Please look at it again.")
	}
	if err := insertEvent(ctx, tx, orderID, e); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ByIdempotencyKey returns the order a key already created, if any.
func (r *Repository) ByIdempotencyKey(ctx context.Context, customerID, key string) (domain.Order, bool, error) {
	var orderID string
	err := r.db.QueryRow(ctx,
		`SELECT order_id FROM order_idempotency_keys WHERE customer_id = $1 AND key = $2`,
		customerID, key).Scan(&orderID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Order{}, false, nil
	}
	if err != nil {
		return domain.Order{}, false, err
	}
	order, err := r.Order(ctx, orderID)
	if err != nil {
		return domain.Order{}, false, err
	}
	return order, true, nil
}

// ClaimIdempotencyKey records that a key belongs to an order.
func (r *Repository) ClaimIdempotencyKey(ctx context.Context, customerID, key, orderID string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO order_idempotency_keys (customer_id, key, order_id)
		VALUES ($1, $2, $3)`, customerID, key, orderID)
	return err
}
