// Package repository defines the transport-neutral Repository Service contract.
//
// Phase 4.0.2 provides immutable values, capability interfaces, stable errors,
// configuration, profiles, and conformance support. It intentionally contains
// no repository lifecycle or scan orchestration implementation.
package repository

import (
	"context"
	"io"
)

// ContractVersion identifies the stable Repository Service contract.
const ContractVersion = "1.0.0"

// RepositoryLifecycleService coordinates repository registration and archival.
type RepositoryLifecycleService interface {
	RegisterRepository(context.Context, RegisterRepositoryRequest) (Repository, error)
	GetRepository(context.Context, RepositoryQuery) (Repository, error)
	ListRepositories(context.Context, RepositoryListRequest) (RepositoryPage, error)
	ArchiveRepository(context.Context, ArchiveRepositoryRequest) (Repository, error)
}

// ScanExecutionService coordinates one synchronous scan lifecycle.
type ScanExecutionService interface {
	ExecuteScan(context.Context, ExecuteScanRequest) (ScanResult, error)
	GetScan(context.Context, ScanQuery) (Scan, error)
	ListScans(context.Context, ScanListRequest) (ScanPage, error)
	CancelScan(context.Context, CancelScanRequest) (Scan, error)
}

// ArtifactQueryService reads metadata and streams exact authoritative bytes.
type ArtifactQueryService interface {
	GetArtifact(context.Context, ArtifactQuery) (Artifact, error)
	ListArtifacts(context.Context, ArtifactListRequest) (ArtifactPage, error)
	ExportArtifact(context.Context, ExportArtifactRequest, io.Writer) (ExportReceipt, error)
}

// Service composes all neutral capabilities for convenience only.
type Service interface {
	RepositoryLifecycleService
	ScanExecutionService
	ArtifactQueryService
}
