// Package app owns non-networked runtime startup, work admission, draining,
// callable health, and graceful shutdown. It owns no API listener, signal
// registration, observability exporter, business logic, or engine behavior.
package app

import (
	"context"

	runtimeconfig "github.com/AjayMunagala/software-engineering-platform/backend/internal/runtime/config"
	runtimehealth "github.com/AjayMunagala/software-engineering-platform/backend/internal/runtime/health"
	runtimepostgres "github.com/AjayMunagala/software-engineering-platform/backend/internal/runtime/postgres"
)

// ContractVersion identifies the candidate Phase 3.5.3 lifecycle contract.
const ContractVersion = "0.1.0"

// PostgreSQLRuntime is the opaque resource capability required by lifecycle.
// Pool internals and SQL are intentionally absent.
type PostgreSQLRuntime interface {
	runtimehealth.DatabaseChecker
	Ingest() runtimepostgres.IngestCapabilities
	Read() runtimepostgres.ReadCapabilities
	Retention() runtimepostgres.RetentionCapabilities
	Close()
}

// PostgreSQLOpener enables deterministic startup failure injection.
type PostgreSQLOpener interface {
	Open(context.Context, runtimeconfig.LoadedConfiguration) (PostgreSQLRuntime, error)
}

// Starter constructs a ready runtime or returns after complete cleanup.
type Starter interface {
	Start(context.Context, runtimeconfig.LoadRequest) (*Runtime, error)
}

// Work is one admitted operation. Done must be called exactly once by the
// consumer; repeated calls are safe.
type Work interface {
	Context() context.Context
	Done()
}
