// Package postgres owns PostgreSQL TLS material, capability-specific pools,
// compatibility proof, adapter construction, and pool cleanup for the runtime.
// It never runs migrations and exposes no pool to engines or application code.
package postgres

import (
	"context"

	runtimeconfig "github.com/AjayMunagala/software-engineering-platform/backend/internal/runtime/config"
	"github.com/AjayMunagala/software-engineering-platform/backend/persistence"
)

// ContractVersion identifies the frozen PostgreSQL Runtime contract.
const ContractVersion = "1.0.0"

// Factory creates and proves a complete PostgreSQL runtime resource set.
type Factory interface {
	Open(context.Context, runtimeconfig.LoadedConfiguration) (*Runtime, error)
}

// Checker is the opaque, read-only runtime capability consumed by health.
// It exposes no pool statistics, SQL, credentials, or driver objects.
type Checker interface {
	Check(context.Context) error
}

// IngestCapabilities is the narrow persistence surface routed to ingestion.
type IngestCapabilities interface {
	persistence.RepositoryStore
	persistence.ScanStore
	persistence.PayloadStager
	persistence.PublicationStore
}

// ReadCapabilities is the narrow exact-artifact retrieval surface.
type ReadCapabilities interface {
	persistence.ArtifactReader
	persistence.IntegrityVerifier
}

// RetentionCapabilities is the narrow retention and garbage-collection surface.
type RetentionCapabilities interface {
	persistence.RetentionStore
}

// Capability names one owned pool without exposing a database identity.
type Capability string

const (
	CapabilityCombined  Capability = "combined"
	CapabilityIngest    Capability = "ingest"
	CapabilityRead      Capability = "read"
	CapabilityRetention Capability = "retention"
)
