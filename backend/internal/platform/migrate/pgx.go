package migrate

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Conn is the pgx surface the adapter needs.
//
// The signatures match pgx exactly, including the concrete pgconn.CommandTag
// return: Go has no covariant returns, so an interface that returned a
// narrower tag type would be satisfied by nothing pgx actually provides.
// *pgx.Conn, pgx.Tx and *pgxpool.Pool all satisfy this as written.
type Conn interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// PgxExecer adapts a pgx connection, transaction or pool to Execer.
type PgxExecer struct {
	conn Conn
}

// NewPgxExecer wraps a pgx connection, transaction or pool.
func NewPgxExecer(conn Conn) *PgxExecer { return &PgxExecer{conn: conn} }

// Exec runs a statement and reports how many rows it touched.
func (p *PgxExecer) Exec(ctx context.Context, sql string, args ...any) (int64, error) {
	tag, err := p.conn.Exec(ctx, sql, args...)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// QueryVersions reads the applied versions and their dirty flags.
//
// An error here is never softened into an empty set: an empty set means "this
// database has no migrations", which would make Up re-apply everything.
func (p *PgxExecer) QueryVersions(ctx context.Context) (map[int]bool, error) {
	rows, err := p.conn.Query(ctx, `SELECT version, dirty FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("select schema_migrations: %w", err)
	}
	defer rows.Close()

	out := map[int]bool{}
	for rows.Next() {
		var version int
		var dirty bool
		if err := rows.Scan(&version, &dirty); err != nil {
			return nil, fmt.Errorf("scan version: %w", err)
		}
		out[version] = dirty
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read versions: %w", err)
	}
	return out, nil
}
