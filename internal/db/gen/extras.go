package db

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Exec exposes an escape hatch to run ad-hoc statements using the underlying DBTX.
func (q *Queries) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	return q.db.Exec(ctx, sql, args...)
}

// QueryRow exposes an escape hatch to run ad-hoc queries using the underlying DBTX.
func (q *Queries) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	return q.db.QueryRow(ctx, sql, args...)
}

// Query exposes an escape hatch to run ad-hoc queries using the underlying DBTX.
func (q *Queries) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	return q.db.Query(ctx, sql, args...)
}
