// Package scan implements transport-, persistence-, runtime-, and
// engine-neutral synchronous scan coordination.
//
// It depends only on the neutral Repository Service contract and narrow
// admission, prepared-analysis, clock, and atomic-store capabilities.
// Intelligence engines, materializers, persistence adapters, runtime wiring,
// database drivers, and transports remain behind those boundaries.
package scan

import (
	"context"
	"io"
	"time"

	"github.com/AjayMunagala/software-engineering-platform/backend/service/repository"
)

const ContractVersion = "1.0.0"

// Store owns atomic scan mutation, idempotency, publication, scope, and query
// semantics. Implementations must consume payload streams during Publish and
// must never retain SourceHandle.
type Store interface {
	Begin(context.Context, BeginCommand) (BeginResult, error)
	Publish(context.Context, PublishCommand) (repository.ScanResult, error)
	Finalize(context.Context, FinalizeCommand) (repository.Scan, error)
	Reconcile(context.Context, repository.Scope, repository.RepositoryID, repository.ScanID) (ReconcileResult, error)
	GetScan(context.Context, repository.Scope, repository.RepositoryID, repository.ScanID) (repository.Scan, error)
	ListScans(context.Context, repository.Scope, repository.RepositoryID, int, repository.Cursor) (ScanList, error)
	Cancel(context.Context, CancelCommand) (repository.Scan, error)
	GetArtifact(context.Context, repository.Scope, repository.RepositoryID, repository.ScanID, repository.ArtifactID) (repository.Artifact, error)
	ListArtifacts(context.Context, repository.Scope, repository.RepositoryID, repository.ScanID, int, repository.Cursor) (ArtifactList, error)
	ExportArtifact(context.Context, repository.Scope, repository.RepositoryID, repository.ScanID, repository.ArtifactID, io.Writer) (repository.ExportReceipt, error)
}

// AdmissionController obtains one bounded work lease for a newly created
// in-process flight. Joined callers never acquire additional leases.
type AdmissionController interface {
	Acquire(context.Context) (WorkLease, error)
}

// WorkLease exposes drain cancellation without leaking runtime internals.
type WorkLease interface {
	Context() context.Context
	Done()
}

// AnalysisPreparer resolves the opaque source and returns one isolated,
// adapter-owned analysis session. Engine-specific behavior remains outside the
// scan coordinator.
type AnalysisPreparer interface {
	Prepare(context.Context, AnalysisRequest) (AnalysisSession, error)
}

// AnalysisSession exposes path-free source proof and prepared artifact inputs.
// Close must respect its bounded context.
type AnalysisSession interface {
	SourceFingerprint() repository.Digest
	SourceRevision() string
	Analyze(context.Context) (AnalysisResult, error)
	Close(context.Context) error
}

// PayloadSource reopens exact sealed analysis bytes.
type PayloadSource interface {
	Open(context.Context) (io.ReadCloser, error)
}

// Clock makes every state transition deterministic under test.
type Clock interface{ Now() time.Time }

type ClockFunc func() time.Time

func (function ClockFunc) Now() time.Time { return function() }

var _ repository.ScanExecutionService = (*Service)(nil)
var _ repository.ArtifactQueryService = (*Service)(nil)
