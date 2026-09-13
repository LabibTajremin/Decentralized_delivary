package platform

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rootlogic-lab/delivery/backend/internal/platform/migrate"
)

// PgxExecer is the only part of the migrator that touches a driver. A real
// database exercises the happy path in the integration suite; these tests drive
// the failures a healthy database never produces, so a dropped connection
// during a deploy surfaces as an error rather than an empty version set — which
// would make the migrator re-apply everything.

// stubRows is a canned pgx.Rows over (version, dirty) pairs.
type stubRows struct {
	rows    [][]any
	idx     int
	scanErr error
	iterErr error
	closed  bool
}

func (s *stubRows) Close()                                       { s.closed = true }
func (s *stubRows) Err() error                                   { return s.iterErr }
func (s *stubRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (s *stubRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (s *stubRows) Values() ([]any, error)                       { return nil, nil }
func (s *stubRows) RawValues() [][]byte                          { return nil }
func (s *stubRows) Conn() *pgx.Conn                              { return nil }
func (s *stubRows) TypeMap() *pgtype.Map                         { return pgtype.NewMap() }

func (s *stubRows) Next() bool {
	if s.idx >= len(s.rows) {
		return false
	}
	s.idx++
	return true
}

func (s *stubRows) Scan(dest ...any) error {
	if s.scanErr != nil {
		return s.scanErr
	}
	values := s.rows[s.idx-1]
	for i, d := range dest {
		switch target := d.(type) {
		case *int:
			*target = values[i].(int)
		case *bool:
			*target = values[i].(bool)
		default:
			return errors.New("stub: unsupported destination type")
		}
	}
	return nil
}

// stubConn scripts a pgx connection.
type stubConn struct {
	execErr  error
	execTag  pgconn.CommandTag
	queryErr error
	rows     *stubRows
	lastSQL  string
	lastArgs []any
}

func (c *stubConn) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	c.lastSQL, c.lastArgs = sql, args
	if c.execErr != nil {
		return pgconn.CommandTag{}, c.execErr
	}
	return c.execTag, nil
}

func (c *stubConn) Query(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
	c.lastSQL = sql
	if c.queryErr != nil {
		return nil, c.queryErr
	}
	if c.rows == nil {
		return &stubRows{}, nil
	}
	return c.rows, nil
}

func TestPgxExecerPassesTheStatementThroughWithItsArguments(t *testing.T) {
	conn := &stubConn{execTag: pgconn.NewCommandTag("DELETE 3")}
	n, err := migrate.NewPgxExecer(conn).Exec(ctx(), `DELETE FROM t WHERE id = $1`, 7)
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if n != 3 {
		t.Errorf("rows affected = %d, want 3", n)
	}
	if conn.lastSQL != `DELETE FROM t WHERE id = $1` {
		t.Errorf("sql = %q", conn.lastSQL)
	}
	if len(conn.lastArgs) != 1 || conn.lastArgs[0] != 7 {
		t.Errorf("args = %v, want [7]", conn.lastArgs)
	}
}

func TestPgxExecerSurfacesAnExecFailure(t *testing.T) {
	conn := &stubConn{execErr: errDB}
	if _, err := migrate.NewPgxExecer(conn).Exec(ctx(), `SELECT 1`); !errors.Is(err, errDB) {
		t.Errorf("error = %v, want the failure wrapped", err)
	}
}

func TestPgxExecerReadsVersionsAndDirtyFlags(t *testing.T) {
	conn := &stubConn{rows: &stubRows{rows: [][]any{{1, false}, {2, true}}}}
	got, err := migrate.NewPgxExecer(conn).QueryVersions(ctx())
	if err != nil {
		t.Fatalf("QueryVersions: %v", err)
	}
	if len(got) != 2 || got[1] != false || got[2] != true {
		t.Errorf("versions = %v, want {1:false 2:true}", got)
	}
	if !conn.rows.closed {
		t.Error("rows must be closed")
	}
}

func TestPgxExecerSurfacesAQueryFailure(t *testing.T) {
	conn := &stubConn{queryErr: errDB}
	if _, err := migrate.NewPgxExecer(conn).QueryVersions(ctx()); !errors.Is(err, errDB) {
		t.Errorf("error = %v, want the failure wrapped", err)
	}
}

func TestPgxExecerSurfacesAScanFailure(t *testing.T) {
	conn := &stubConn{rows: &stubRows{rows: [][]any{{1, false}}, scanErr: errDB}}
	if _, err := migrate.NewPgxExecer(conn).QueryVersions(ctx()); !errors.Is(err, errDB) {
		t.Errorf("error = %v, want the failure wrapped", err)
	}
}

// TestPgxExecerSurfacesAnIterationFailure matters most: a connection that drops
// mid-stream yields a short, error-free-looking version set, and treating that
// as authoritative would re-apply migrations already in the database.
func TestPgxExecerSurfacesAnIterationFailure(t *testing.T) {
	conn := &stubConn{rows: &stubRows{iterErr: errDB}}
	if _, err := migrate.NewPgxExecer(conn).QueryVersions(ctx()); !errors.Is(err, errDB) {
		t.Errorf("error = %v, want the failure wrapped", err)
	}
}

// The adapter is useless if the real driver types do not satisfy Conn, and a
// stub that satisfies it proves nothing about pgx. These assertions are the
// check that matters: an interface whose signatures drift from pgx's — a
// narrowed return type, say — fails to compile here rather than at wiring time.
var (
	_ migrate.Conn = (*pgx.Conn)(nil)
	_ migrate.Conn = (*pgxpool.Pool)(nil)
	_ migrate.Conn = (pgx.Tx)(nil)

	_ migrate.Execer = (*migrate.PgxExecer)(nil)
)
