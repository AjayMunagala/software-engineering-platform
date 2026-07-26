package repositoryservice

import (
	"context"
	"io"
	"os"
	"sync"

	"github.com/AjayMunagala/software-engineering-platform/backend/lie"
	goengine "github.com/AjayMunagala/software-engineering-platform/backend/lie/golang"
	"github.com/AjayMunagala/software-engineering-platform/backend/lie/golang/packageidentity"
	"github.com/AjayMunagala/software-engineering-platform/backend/lie/golang/semantic"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
)

const (
	// IdentityScheme is the frozen spike candidate for deterministic artifact IDs.
	IdentityScheme = "repository-service-artifact-id/v1"
	identityPrefix = "rsaid1_"
)

// IdentityInput contains every value covered by a deterministic artifact ID.
type IdentityInput struct {
	RepositoryID    string
	ScanID          string
	ArtifactName    string
	ArtifactVersion string
	StableIDScheme  string
}

// ArtifactDescriptor records exact sealed payload facts.
type ArtifactDescriptor struct {
	ArtifactID     string
	Name           string
	Version        string
	StableIDScheme string
	PayloadDigest  string
	PayloadSize    uint64
}

// SealedArtifact is a reopenable exact-byte spool. Its path is intentionally
// private and never part of a durable or observable model.
type SealedArtifact struct {
	descriptor ArtifactDescriptor
	path       string
	bufferSize int
	mu         sync.Mutex
	closed     bool
}

func (artifact *SealedArtifact) Descriptor() ArtifactDescriptor {
	if artifact == nil {
		return ArtifactDescriptor{}
	}
	return artifact.descriptor
}

// Open returns a new reader over the sealed bytes.
func (artifact *SealedArtifact) Open(ctx context.Context) (io.ReadCloser, error) {
	if ctx == nil {
		return nil, ErrContextRequired
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if artifact == nil {
		return nil, ErrArtifactClosed
	}
	artifact.mu.Lock()
	defer artifact.mu.Unlock()
	if artifact.closed {
		return nil, ErrArtifactClosed
	}
	return os.Open(artifact.path)
}

// Close removes the spool exactly once.
func (artifact *SealedArtifact) Close(context.Context) error {
	if artifact == nil {
		return nil
	}
	artifact.mu.Lock()
	defer artifact.mu.Unlock()
	if artifact.closed {
		return nil
	}
	artifact.closed = true
	if err := os.Remove(artifact.path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// PublicationState is the neutral durable scan state used by reconciliation.
type PublicationState string

const (
	PublicationRunning   PublicationState = "running"
	PublicationSucceeded PublicationState = "succeeded"
	PublicationFailed    PublicationState = "failed"
	PublicationCanceled  PublicationState = "canceled"
)

// PublicationOutcome distinguishes an ordinary success from a reconciled one.
type PublicationOutcome struct {
	Published  bool
	Reconciled bool
}

// FlightDisposition records whether the caller created or joined an execution.
type FlightDisposition string

const (
	FlightCreated FlightDisposition = "created"
	FlightJoined  FlightDisposition = "joined"
)

// GoLanguageInventoryView is the explicit detached durable candidate missing
// from the released syntax artifact. It changes no released contract.
type GoLanguageInventoryView struct {
	Artifact        goengine.Metadata        `json:"artifact"`
	SourceArtifacts []rie.ArtifactReference  `json:"source_artifacts"`
	Files           []goengine.GoFile        `json:"files"`
	Packages        []goengine.GoPackage     `json:"packages"`
	Symbols         []goengine.GoSymbol      `json:"symbols"`
	Diagnostics     []lie.Diagnostic         `json:"diagnostics"`
	Statistics      goengine.ParseStatistics `json:"statistics"`
}

// SpikeAnalysis contains released immutable outputs used only as spike evidence.
type SpikeAnalysis struct {
	Report            rie.Report
	Syntax            goengine.GoLanguageInventory
	PackageIdentities packageidentity.GoPackageIdentityInventory
	Semantics         semantic.GoSemanticInventory
	GoPresent         bool
}
