// Package semantic provides deterministic Go semantic artifacts.
package semantic

import (
	"context"

	"github.com/AjayMunagala/software-engineering-platform/backend/lie/golang"
	"github.com/AjayMunagala/software-engineering-platform/backend/lie/golang/packageidentity"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
)

// Input carries the immutable artifacts required by semantic resolution.
type Input struct {
	Snapshot          rie.RepositorySnapshot
	Syntax            golang.GoLanguageInventory
	PackageIdentities packageidentity.GoPackageIdentityInventory
}

// Engine verifies authorized source and produces a semantic candidate artifact.
type Engine interface {
	Name() string
	Version() string
	Language() string
	ArtifactName() string
	Description() string
	Resolve(context.Context, Input) (GoSemanticInventory, error)
}

// Integrator resolves a fresh semantic candidate from typed prerequisites and
// publishes it exactly once through the per-run artifact store.
type Integrator interface {
	Run(context.Context, *rie.ArtifactStore) (GoSemanticInventory, error)
}

// New returns a semantic engine configured with defaults or one explicit config.
func New(configs ...Config) (Engine, error) {
	if len(configs) > 1 {
		return nil, ErrTooManyConfigs
	}
	config := DefaultConfig()
	if len(configs) == 1 {
		config = configs[0].withDefaults()
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return &engine{config: config}, nil
}

// NewIntegrator returns the additive Phase 2.2.7 artifact-store integration.
// Every Run performs a full rebuild; the integrator retains no semantic state.
func NewIntegrator(configs ...Config) (Integrator, error) {
	resolver, err := New(configs...)
	if err != nil {
		return nil, err
	}
	return &integrator{resolver: resolver}, nil
}

// InventoryFrom retrieves a semantic artifact from a per-run artifact store.
func InventoryFrom(store *rie.ArtifactStore) (GoSemanticInventory, bool) {
	return rie.ArtifactAs[GoSemanticInventory](store, ArtifactName)
}
