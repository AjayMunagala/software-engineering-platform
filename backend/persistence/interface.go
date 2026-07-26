// Package persistence defines the storage-neutral durable artifact contract.
//
// The package has no database driver, SQL, engine, artifact, environment, or
// runtime-configuration dependency. Application orchestration supplies
// detached metadata and exact byte streams.
package persistence

import (
	"context"
	"io"
)

// ContractVersion identifies the frozen storage-neutral API contract.
const ContractVersion = "1.0.0"

// RepositoryStore owns repository lifecycle records.
type RepositoryStore interface {
	RegisterRepository(context.Context, RegisterRepositoryRequest) (RepositoryRecord, error)
	GetRepository(context.Context, RepositoryQuery) (RepositoryRecord, error)
	ListRepositories(context.Context, RepositoryListRequest) (RepositoryPage, error)
	ArchiveRepository(context.Context, ArchiveRepositoryRequest) (RepositoryRecord, error)
}

// ScanStore owns scan lifecycle records.
type ScanStore interface {
	BeginScan(context.Context, BeginScanRequest) (ScanRecord, error)
	GetScan(context.Context, ScanQuery) (ScanRecord, error)
	ListScans(context.Context, ScanListRequest) (ScanPage, error)
	FailScan(context.Context, FinishScanRequest) (ScanRecord, error)
	CancelScan(context.Context, FinishScanRequest) (ScanRecord, error)
}

// PayloadStager consumes and verifies one exact immutable payload.
type PayloadStager interface {
	StagePayload(context.Context, StagePayloadRequest, io.Reader) (PayloadReceipt, error)
}

// PublicationStore atomically publishes one complete scan manifest.
type PublicationStore interface {
	PublishScan(context.Context, PublishScanRequest) (PublicationReceipt, error)
}

// ArtifactReader retrieves immutable envelope metadata and exact payload bytes.
type ArtifactReader interface {
	GetArtifact(context.Context, ArtifactQuery) (ArtifactRecord, error)
	ListArtifacts(context.Context, ArtifactListRequest) (ArtifactPage, error)
	ExportPayload(context.Context, PayloadQuery, io.Writer) (PayloadReceipt, error)
}

// IntegrityVerifier recomputes stored payload integrity evidence.
type IntegrityVerifier interface {
	VerifyPayload(context.Context, PayloadQuery) (VerificationReceipt, error)
}

// RetentionStore owns archival purge and unreferenced-payload collection.
type RetentionStore interface {
	MarkRepositoryForPurge(context.Context, MarkForPurgeRequest) (RepositoryRecord, error)
	PurgeRepositoryBatch(context.Context, PurgeBatchRequest) (PurgeReceipt, error)
	GarbageCollectPayloads(context.Context, GarbageCollectionRequest) (GarbageCollectionReceipt, error)
}

// Port composes every neutral persistence capability. Consumers should prefer
// the smallest capability interface that satisfies their use case.
type Port interface {
	RepositoryStore
	ScanStore
	PayloadStager
	PublicationStore
	ArtifactReader
	IntegrityVerifier
	RetentionStore
}
