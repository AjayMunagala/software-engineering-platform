package conformance

import (
	"context"

	"github.com/AjayMunagala/software-engineering-platform/backend/persistence"
)

// Scenario identifies the pre-seeded publication used by all adapter-neutral
// checks. Exact payload bytes must match Digest and PayloadSize.
type Scenario struct {
	PrimaryScope  persistence.Scope
	OtherScope    persistence.Scope
	RepositoryID  persistence.RepositoryID
	ScanID        persistence.ScanID
	ArtifactID    persistence.ArtifactID
	PublicationID persistence.PublicationID
	Artifact      persistence.VersionedName
	Producer      persistence.VersionedName
	Codec         persistence.Codec
	Digest        persistence.Digest
	Payload       []byte
	Actor         persistence.AuditActor
}

// Fixture contains an isolated adapter and stable seeded scenario.
type Fixture struct {
	Port     persistence.Port
	Contract *persistence.Contract
	Scenario Scenario
	context  context.Context
}

// Operation names every public capability covered by the scope-isolation
// conformance gate.
type Operation string

const (
	OperationRegisterRepository Operation = "register-repository"
	OperationGetRepository      Operation = "get-repository"
	OperationListRepositories   Operation = "list-repositories"
	OperationArchiveRepository  Operation = "archive-repository"
	OperationBeginScan          Operation = "begin-scan"
	OperationGetScan            Operation = "get-scan"
	OperationListScans          Operation = "list-scans"
	OperationFailScan           Operation = "fail-scan"
	OperationCancelScan         Operation = "cancel-scan"
	OperationStagePayload       Operation = "stage-payload"
	OperationPublishScan        Operation = "publish-scan"
	OperationGetArtifact        Operation = "get-artifact"
	OperationListArtifacts      Operation = "list-artifacts"
	OperationExportPayload      Operation = "export-payload"
	OperationVerifyPayload      Operation = "verify-payload"
	OperationMarkForPurge       Operation = "mark-for-purge"
	OperationPurgeBatch         Operation = "purge-batch"
	OperationGarbageCollect     Operation = "garbage-collect"
)

// ScopeIsolationOperations returns every public operation exactly once.
func ScopeIsolationOperations() []Operation {
	return []Operation{
		OperationRegisterRepository, OperationGetRepository,
		OperationListRepositories, OperationArchiveRepository,
		OperationBeginScan, OperationGetScan, OperationListScans,
		OperationFailScan, OperationCancelScan, OperationStagePayload,
		OperationPublishScan, OperationGetArtifact, OperationListArtifacts,
		OperationExportPayload, OperationVerifyPayload, OperationMarkForPurge,
		OperationPurgeBatch, OperationGarbageCollect,
	}
}

func (scenario Scenario) clone() Scenario {
	result := scenario
	result.Payload = append([]byte(nil), scenario.Payload...)
	return result
}
