// Package scan implements Phase 4.0.4 synchronous scan coordination.
//
// It depends only on the neutral Repository Service contract and narrow
// admission, prepared-analysis, clock, and atomic-store capabilities. Real
// intelligence engines, materializers, persistence adapters, runtime wiring,
// database drivers, and transports are deliberately outside this package.
package scan

import (
	"context"
	"io"
	"time"

	"github.com/AjayMunagala/software-engineering-platform/backend/service/repository"
)

const ContractVersion = "0.1.0"

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

// AnalysisPreparer resolves the opaque source and returns one isolated fake or
// future adapter-owned analysis session. Phase 4.0.4 implementations must not
// execute the real RIE/LIE pipeline.
type AnalysisPreparer interface {
	Prepare(context.Context, AnalysisRequest) (AnalysisSession, error)
}

// AnalysisSession exposes path-free source proof and already-prepared fake
// artifact inputs. Close must respect its bounded context.
type AnalysisSession interface {
	SourceFingerprint() repository.Digest
	SourceRevision() string
	Analyze(context.Context) (AnalysisResult, error)
	Close(context.Context) error
}

// PayloadSource reopens exact fake-analysis bytes. Real materialization is a
// later milestone.
type PayloadSource interface {
	Open(context.Context) (io.ReadCloser, error)
}

// Clock makes every state transition deterministic under test.
type Clock interface{ Now() time.Time }

type ClockFunc func() time.Time

func (function ClockFunc) Now() time.Time { return function() }

var _ repository.ScanExecutionService = (*Service)(nil)
var _ repository.ArtifactQueryService = (*Service)(nil)
