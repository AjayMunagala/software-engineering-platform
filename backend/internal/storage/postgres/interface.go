// Package postgres implements the storage-neutral persistence port against
// the accepted PostgreSQL physical schema.
package postgres

import (
	"context"

	"github.com/AjayMunagala/software-engineering-platform/backend/persistence"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// AdapterVersion identifies the frozen PostgreSQL reference adapter contract.
const AdapterVersion = "1.0.0"

var _ persistence.Port = (*Adapter)(nil)

// Database is the smallest pgx-compatible capability required by the adapter.
// A *pgxpool.Pool satisfies it; pool construction and runtime configuration
// deliberately remain outside this package.
type Database interface {
	BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error)
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}
