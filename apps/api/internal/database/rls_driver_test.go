package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"
)

type recordedOp struct {
	kind  string
	query string
	args  []any
}

type fakeRLSDriver struct {
	mu  sync.Mutex
	ops []recordedOp
}

func (d *fakeRLSDriver) record(op recordedOp) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.ops = append(d.ops, op)
}

func (d *fakeRLSDriver) snapshot() []recordedOp {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]recordedOp, len(d.ops))
	copy(out, d.ops)
	return out
}

func (d *fakeRLSDriver) Open(name string) (driver.Conn, error) {
	return &fakeConn{drv: d}, nil
}

type fakeConnector struct {
	drv        *fakeRLSDriver
	useErrSkip bool
}

func (c *fakeConnector) Connect(ctx context.Context) (driver.Conn, error) {
	return &fakeConn{drv: c.drv, useErrSkip: c.useErrSkip}, nil
}

func (c *fakeConnector) Driver() driver.Driver {
	return c.drv
}

type fakeConn struct {
	drv        *fakeRLSDriver
	useErrSkip bool
}

func namedValuesToAny(args []driver.NamedValue) []any {
	out := make([]any, len(args))
	for i, a := range args {
		out[i] = a.Value
	}
	return out
}

func (c *fakeConn) Prepare(query string) (driver.Stmt, error) {
	return &fakeStmt{drv: c.drv, query: query}, nil
}

func (c *fakeConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	return &fakeStmt{drv: c.drv, query: query}, nil
}

func (c *fakeConn) Close() error { return nil }

func (c *fakeConn) Begin() (driver.Tx, error) {
	c.drv.record(recordedOp{kind: "begin"})
	return &fakeTx{drv: c.drv}, nil
}

func (c *fakeConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	c.drv.record(recordedOp{kind: "begin_tx"})
	return &fakeTx{drv: c.drv}, nil
}

func (c *fakeConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if c.useErrSkip {
		return nil, driver.ErrSkip
	}
	c.drv.record(recordedOp{kind: "exec", query: query, args: namedValuesToAny(args)})
	return driver.RowsAffected(1), nil
}

func (c *fakeConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	c.drv.record(recordedOp{kind: "query", query: query, args: namedValuesToAny(args)})
	return &fakeRows{}, nil
}

func (c *fakeConn) ResetSession(ctx context.Context) error { return nil }

type fakeStmt struct {
	drv   *fakeRLSDriver
	query string
}

func (s *fakeStmt) Close() error  { return nil }
func (s *fakeStmt) NumInput() int { return -1 }

func (s *fakeStmt) Exec(args []driver.Value) (driver.Result, error) {
	out := make([]any, len(args))
	for i, v := range args {
		out[i] = v
	}
	s.drv.record(recordedOp{kind: "stmt_exec", query: s.query, args: out})
	return driver.RowsAffected(1), nil
}

func (s *fakeStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	s.drv.record(recordedOp{kind: "stmt_exec", query: s.query, args: namedValuesToAny(args)})
	return driver.RowsAffected(1), nil
}

func (s *fakeStmt) Query(args []driver.Value) (driver.Rows, error) {
	return &fakeRows{}, nil
}

func (s *fakeStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	s.drv.record(recordedOp{kind: "stmt_query", query: s.query, args: namedValuesToAny(args)})
	return &fakeRows{}, nil
}

type fakeTx struct {
	drv *fakeRLSDriver
}

func (t *fakeTx) Commit() error {
	t.drv.record(recordedOp{kind: "commit"})
	return nil
}

func (t *fakeTx) Rollback() error {
	t.drv.record(recordedOp{kind: "rollback"})
	return nil
}

type fakeRows struct{}

func (r *fakeRows) Columns() []string              { return []string{} }
func (r *fakeRows) Close() error                   { return nil }
func (r *fakeRows) Next(dest []driver.Value) error { return driver.ErrSkip }

func TestRLSConnector_PoolReuseAndReset(t *testing.T) {
	drv := &fakeRLSDriver{}
	db := sql.OpenDB(NewRLSConnector(&fakeConnector{drv: drv}))
	defer db.Close()
	db.SetMaxOpenConns(1)

	aliceCtx := WithTenantContext(context.Background(), TenantDBContext{UserID: "u_alice", OrgID: "org_a"})
	bobCtx := WithTenantContext(context.Background(), TenantDBContext{UserID: "u_bob", OrgID: "org_b"})

	// 1. Alice first query sets GUCs before query.
	if _, err := db.ExecContext(aliceCtx, "SELECT 1"); err != nil {
		t.Fatalf("alice first exec: %v", err)
	}
	// 2. Alice second query on the same pooled connection skips redundant set_config.
	if _, err := db.ExecContext(aliceCtx, "SELECT 2"); err != nil {
		t.Fatalf("alice second exec: %v", err)
	}
	// 3. Bob query switches session GUCs before executing.
	if _, err := db.ExecContext(bobCtx, "SELECT 3"); err != nil {
		t.Fatalf("bob exec: %v", err)
	}
	// 4. Non-tenant background context resets session GUCs back to empty strings.
	if _, err := db.ExecContext(context.Background(), "SELECT 4"); err != nil {
		t.Fatalf("background exec: %v", err)
	}
	// 5. System bypass context enables explicit app.rls_bypass='on'.
	if _, err := db.ExecContext(WithSystemBypassRLS(context.Background()), "SELECT 4.5"); err != nil {
		t.Fatalf("system bypass exec: %v", err)
	}
	// 6. Subsequent tenant query resets bypass back to ''.
	if _, err := db.ExecContext(aliceCtx, "SELECT 4.75"); err != nil {
		t.Fatalf("alice post-bypass exec: %v", err)
	}
	// 7. Transaction rollback invalidates cached GUC state so next query re-applies cleanly.
	tx, err := db.BeginTx(aliceCtx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if _, err := db.ExecContext(aliceCtx, "SELECT 5"); err != nil {
		t.Fatalf("alice post-rollback exec: %v", err)
	}

	want := []recordedOp{
		{kind: "exec", query: setRLSConfigSQL, args: []any{"u_alice", "org_a", ""}},
		{kind: "exec", query: "SELECT 1", args: []any{}},
		{kind: "exec", query: "SELECT 2", args: []any{}},
		{kind: "exec", query: setRLSConfigSQL, args: []any{"u_bob", "org_b", ""}},
		{kind: "exec", query: "SELECT 3", args: []any{}},
		{kind: "exec", query: setRLSConfigSQL, args: []any{"", "", ""}},
		{kind: "exec", query: "SELECT 4", args: []any{}},
		{kind: "exec", query: setRLSConfigSQL, args: []any{"", "", "on"}},
		{kind: "exec", query: "SELECT 4.5", args: []any{}},
		{kind: "exec", query: setRLSConfigSQL, args: []any{"u_alice", "org_a", ""}},
		{kind: "exec", query: "SELECT 4.75", args: []any{}},
		{kind: "begin_tx"},
		{kind: "rollback"},
		{kind: "exec", query: setRLSConfigSQL, args: []any{"u_alice", "org_a", ""}},
		{kind: "exec", query: "SELECT 5", args: []any{}},
	}

	got := drv.snapshot()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("recorded operations mismatch:\ngot:  %+v\nwant: %+v", got, want)
	}
}

func TestRLSConnector_PreparedStmtFallback(t *testing.T) {
	drv := &fakeRLSDriver{}
	db := sql.OpenDB(NewRLSConnector(&fakeConnector{drv: drv, useErrSkip: true}))
	defer db.Close()
	db.SetMaxOpenConns(1)

	ctx := WithTenantContext(context.Background(), TenantDBContext{UserID: "u_stmt", OrgID: "org_stmt"})
	if _, err := db.ExecContext(ctx, "UPDATE foo SET bar = $1", "baz"); err != nil {
		t.Fatalf("exec via prepared fallback: %v", err)
	}

	got := drv.snapshot()
	want := []recordedOp{
		{kind: "stmt_exec", query: setRLSConfigSQL, args: []any{"u_stmt", "org_stmt", ""}},
		{kind: "stmt_exec", query: "UPDATE foo SET bar = $1", args: []any{"baz"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("prepared fallback ops mismatch:\ngot:  %+v\nwant: %+v", got, want)
	}
}

func TestRLSLivePostgresPoolIsolation(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		t.Skip("DATABASE_URL not configured; skipping live PostgreSQL RLS session test")
	}

	db, err := NewPostgres(dsn, 5*time.Second)
	if err != nil {
		t.Skipf("PostgreSQL unavailable: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	readGUCs := func(ctx context.Context) (string, string, string) {
		t.Helper()
		var uid, oid, bypass sql.NullString
		err := db.QueryRowContext(ctx, "SELECT current_setting('app.current_user_id', true), current_setting('app.current_org_id', true), current_setting('app.rls_bypass', true)").Scan(&uid, &oid, &bypass)
		if err != nil {
			t.Fatalf("query current_setting: %v", err)
		}
		return uid.String, oid.String, bypass.String
	}

	u, o, b := readGUCs(WithTenantContext(context.Background(), TenantDBContext{UserID: "u_1", OrgID: "org_1"}))
	if u != "u_1" || o != "org_1" || b != "" {
		t.Fatalf("expected (u_1, org_1, ''), got (%q, %q, %q)", u, o, b)
	}
	u, o, b = readGUCs(WithSystemBypassRLS(context.Background()))
	if u != "" || o != "" || b != "on" {
		t.Fatalf("expected ('', '', 'on'), got (%q, %q, %q)", u, o, b)
	}
	u, o, b = readGUCs(context.Background())
	if u != "" || o != "" || b != "" {
		t.Fatalf("expected reset GUCs ('', '', ''), got (%q, %q, %q)", u, o, b)
	}
}
