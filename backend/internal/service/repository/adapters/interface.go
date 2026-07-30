// Package adapters implements Phase 4.0.5 intelligence and materialization
// adapters for the frozen repository-go/v1 analysis profile.
//
// It is the only production package in this milestone allowed to know the
// released RIE and Go LIE contracts. It does not import persistence, database,
// runtime, transport, authentication, UI, or AI packages.
package adapters

import (
	"context"
	"io"

	"github.com/AjayMunagala/software-engineering-platform/backend/service/repository"
	"github.com/AjayMunagala/software-engineering-platform/backend/service/repository/scan"
)

const ContractVersion = "1.0.0"

// SourceResolver resolves a process-local opaque handle without fetching or
// cloning a repository. Implementations own authorization before returning.
type SourceResolver interface {
	Resolve(context.Context, repository.Scope, repository.SourceHandle) (AuthorizedSource, error)
}

// AuthorizedSource exposes a local root only inside this internal adapter.
// RootPath must never appear in artifacts, service errors, logs, or metrics.
type AuthorizedSource interface {
	RootPath() string
	Fingerprint() repository.Digest
	Revision() string
	Close(context.Context) error
}

// EncodeFunc writes one deterministic artifact representation exactly once.
type EncodeFunc func(context.Context, io.Writer) error

var _ scan.AnalysisPreparer = (*Adapter)(nil)
var _ scan.AnalysisSession = (*session)(nil)
var _ scan.PayloadSource = (*sealedPayload)(nil)
