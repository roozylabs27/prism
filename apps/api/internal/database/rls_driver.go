package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"sync"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

const setRLSConfigSQL = "SELECT set_config('app.current_user_id', $1, false), set_config('app.current_org_id', $2, false), set_config('app.rls_bypass', $3, false)"

// OpenRLSPostgres opens a PostgreSQL *sqlx.DB backed by pq.NewConnector wrapped
// in NewRLSConnector so pooled connections synchronize RLS session settings.
func OpenRLSPostgres(dsn string) (*sqlx.DB, error) {
	connector, err := pq.NewConnector(dsn)
	if err != nil {
		return nil, err
	}
	return sqlx.NewDb(sql.OpenDB(NewRLSConnector(connector)), "postgres"), nil
}

// NewRLSConnector wraps an underlying driver.Connector so every query or
// transaction synchronizes PostgreSQL session RLS settings from ctx.
func NewRLSConnector(inner driver.Connector) driver.Connector {
	return &rlsConnector{inner: inner}
}

type rlsConnector struct {
	inner driver.Connector
}

func (c *rlsConnector) Connect(ctx context.Context) (driver.Conn, error) {
	raw, err := c.inner.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &rlsConn{inner: raw}, nil
}

func (c *rlsConnector) Driver() driver.Driver {
	return c.inner.Driver()
}

type rlsConn struct {
	inner         driver.Conn
	mu            sync.Mutex
	appliedValid  bool
	appliedUserID string
	appliedOrgID  string
	appliedBypass string
	inTx          bool
}

func (c *rlsConn) ensureRLS(ctx context.Context) error {
	target := TenantContextFrom(ctx)
	bypassVal := ""
	if target.BypassRLS {
		bypassVal = "on"
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.appliedValid && c.appliedUserID == target.UserID && c.appliedOrgID == target.OrgID && c.appliedBypass == bypassVal {
		return nil
	}

	args := []driver.NamedValue{
		{Ordinal: 1, Value: target.UserID},
		{Ordinal: 2, Value: target.OrgID},
		{Ordinal: 3, Value: bypassVal},
	}
	if err := execOnInner(ctx, c.inner, setRLSConfigSQL, args); err != nil {
		// Invalidate cached state so the next query does not trust stale session GUCs.
		c.appliedValid = false
		c.appliedUserID = ""
		c.appliedOrgID = ""
		c.appliedBypass = ""
		return err
	}

	c.appliedValid = true
	c.appliedUserID = target.UserID
	c.appliedOrgID = target.OrgID
	c.appliedBypass = bypassVal
	return nil
}

func execOnInner(ctx context.Context, inner driver.Conn, query string, args []driver.NamedValue) error {
	if execer, ok := inner.(driver.ExecerContext); ok {
		_, err := execer.ExecContext(ctx, query, args)
		if err != driver.ErrSkip {
			return err
		}
	}

	var stmt driver.Stmt
	var err error
	if prepCtx, ok := inner.(driver.ConnPrepareContext); ok {
		stmt, err = prepCtx.PrepareContext(ctx, query)
	} else {
		stmt, err = inner.Prepare(query)
	}
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()

	if stmtExecCtx, ok := stmt.(driver.StmtExecContext); ok {
		_, err = stmtExecCtx.ExecContext(ctx, args)
		if err != driver.ErrSkip {
			return err
		}
	}

	values := make([]driver.Value, len(args))
	for i, arg := range args {
		values[i] = arg.Value
	}
	_, err = stmt.Exec(values) //nolint:staticcheck // fallback for drivers lacking StmtExecContext
	return err
}

func (c *rlsConn) Prepare(query string) (driver.Stmt, error) {
	if err := c.ensureRLS(context.Background()); err != nil {
		return nil, err
	}
	stmt, err := c.inner.Prepare(query)
	if err != nil {
		return nil, err
	}
	return &rlsStmt{inner: stmt, conn: c}, nil
}

func (c *rlsConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	if err := c.ensureRLS(ctx); err != nil {
		return nil, err
	}
	var stmt driver.Stmt
	var err error
	if prepCtx, ok := c.inner.(driver.ConnPrepareContext); ok {
		stmt, err = prepCtx.PrepareContext(ctx, query)
	} else {
		stmt, err = c.inner.Prepare(query)
	}
	if err != nil {
		return nil, err
	}
	return &rlsStmt{inner: stmt, conn: c}, nil
}

func (c *rlsConn) Close() error {
	return c.inner.Close()
}

func (c *rlsConn) Begin() (driver.Tx, error) {
	if err := c.ensureRLS(context.Background()); err != nil {
		return nil, err
	}
	tx, err := c.inner.Begin() //nolint:staticcheck // fallback for drivers lacking ConnBeginTx
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.inTx = true
	c.mu.Unlock()
	return &rlsTx{inner: tx, conn: c}, nil
}

func (c *rlsConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if err := c.ensureRLS(ctx); err != nil {
		return nil, err
	}
	var tx driver.Tx
	var err error
	if beginTx, ok := c.inner.(driver.ConnBeginTx); ok {
		tx, err = beginTx.BeginTx(ctx, opts)
	} else {
		tx, err = c.inner.Begin() //nolint:staticcheck // fallback for drivers lacking ConnBeginTx
	}
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.inTx = true
	c.mu.Unlock()
	return &rlsTx{inner: tx, conn: c}, nil
}

func (c *rlsConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	execer, ok := c.inner.(driver.ExecerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	if err := c.ensureRLS(ctx); err != nil {
		return nil, err
	}
	return execer.ExecContext(ctx, query, args)
}

func (c *rlsConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	queryer, ok := c.inner.(driver.QueryerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	if err := c.ensureRLS(ctx); err != nil {
		return nil, err
	}
	return queryer.QueryContext(ctx, query, args)
}

func (c *rlsConn) Ping(ctx context.Context) error {
	if pinger, ok := c.inner.(driver.Pinger); ok {
		return pinger.Ping(ctx)
	}
	return nil
}

func (c *rlsConn) ResetSession(ctx context.Context) error {
	if resetter, ok := c.inner.(driver.SessionResetter); ok {
		if err := resetter.ResetSession(ctx); err != nil {
			c.mu.Lock()
			c.appliedValid = false
			c.appliedUserID = ""
			c.appliedOrgID = ""
			c.appliedBypass = ""
			c.inTx = false
			c.mu.Unlock()
			return err
		}
	}

	c.mu.Lock()
	inTx := c.inTx
	c.mu.Unlock()
	if !inTx {
		if err := c.ensureRLS(ctx); err != nil {
			// Discard the pooled connection rather than risking stale tenant GUCs.
			return driver.ErrBadConn
		}
	}
	return nil
}

func (c *rlsConn) IsValid() bool {
	if validator, ok := c.inner.(driver.Validator); ok {
		return validator.IsValid()
	}
	return true
}

func (c *rlsConn) CheckNamedValue(nv *driver.NamedValue) error {
	if checker, ok := c.inner.(driver.NamedValueChecker); ok {
		return checker.CheckNamedValue(nv)
	}
	return driver.ErrSkip
}

type rlsStmt struct {
	inner driver.Stmt
	conn  *rlsConn
}

func (s *rlsStmt) Close() error {
	return s.inner.Close()
}

func (s *rlsStmt) NumInput() int {
	return s.inner.NumInput()
}

func (s *rlsStmt) Exec(args []driver.Value) (driver.Result, error) {
	if err := s.conn.ensureRLS(context.Background()); err != nil {
		return nil, err
	}
	return s.inner.Exec(args) //nolint:staticcheck // delegated legacy driver.Stmt method
}

func (s *rlsStmt) Query(args []driver.Value) (driver.Rows, error) {
	if err := s.conn.ensureRLS(context.Background()); err != nil {
		return nil, err
	}
	return s.inner.Query(args) //nolint:staticcheck // delegated legacy driver.Stmt method
}

func (s *rlsStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	if err := s.conn.ensureRLS(ctx); err != nil {
		return nil, err
	}
	if execCtx, ok := s.inner.(driver.StmtExecContext); ok {
		return execCtx.ExecContext(ctx, args)
	}
	values := make([]driver.Value, len(args))
	for i, arg := range args {
		values[i] = arg.Value
	}
	return s.inner.Exec(values) //nolint:staticcheck // fallback for drivers lacking StmtExecContext
}

func (s *rlsStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	if err := s.conn.ensureRLS(ctx); err != nil {
		return nil, err
	}
	if queryCtx, ok := s.inner.(driver.StmtQueryContext); ok {
		return queryCtx.QueryContext(ctx, args)
	}
	values := make([]driver.Value, len(args))
	for i, arg := range args {
		values[i] = arg.Value
	}
	return s.inner.Query(values) //nolint:staticcheck // fallback for drivers lacking StmtQueryContext
}

type rlsTx struct {
	inner driver.Tx
	conn  *rlsConn
}

func (t *rlsTx) Commit() error {
	err := t.inner.Commit()
	t.conn.mu.Lock()
	t.conn.inTx = false
	if err != nil {
		t.conn.appliedValid = false
	}
	t.conn.mu.Unlock()
	return err
}

func (t *rlsTx) Rollback() error {
	err := t.inner.Rollback()
	// PostgreSQL rolls back session GUC changes applied within a rolled-back transaction,
	// so force re-synchronization before this connection serves the next call.
	t.conn.mu.Lock()
	t.conn.inTx = false
	t.conn.appliedValid = false
	t.conn.mu.Unlock()
	return err
}
