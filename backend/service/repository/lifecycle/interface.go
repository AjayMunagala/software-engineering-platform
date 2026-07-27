// Package lifecycle implements Phase 4.0.3 repository lifecycle coordination.
//
// It depends only on the neutral Repository Service contract and narrow
// source-proof, clock, and atomic-store capabilities. It contains no scan
// execution, engine, persistence-adapter, runtime, filesystem, or transport
// implementation.
package lifecycle

import (
	"context"
	"time"

	"github.com/AjayMunagala/software-engineering-platform/backend/service/repository"
)

const ContractVersion = "0.1.0"

// Store owns atomic repository mutation, idempotency, scope, and pagination
// persistence semantics. Implementations must never persist SourceHandle.
type Store interface {
	Register(context.Context, RegisterCommand) (repository.Repository, error)
	Get(context.Context, repository.Scope, repository.RepositoryID) (repository.Repository, error)
	List(context.Context, repository.Scope, int, repository.Cursor) (RepositoryList, error)
	Archive(context.Context, ArchiveCommand) (repository.Repository, error)
}

// SourceProofResolver converts one process-local opaque handle into durable,
// path-free identity evidence. Resolution performs no remote fetch.
type SourceProofResolver interface {
	Resolve(context.Context, repository.Scope, repository.SourceHandle) (SourceResolution, error)
}

// SourceResolution owns any bounded resolver resource and exposes only proof.
type SourceResolution interface {
	Proof() SourceProof
	Close(context.Context) error
}

// Clock makes lifecycle timestamps deterministic under test.
type Clock interface{ Now() time.Time }

type ClockFunc func() time.Time

func (function ClockFunc) Now() time.Time { return function() }

var _ repository.RepositoryLifecycleService = (*Service)(nil)
