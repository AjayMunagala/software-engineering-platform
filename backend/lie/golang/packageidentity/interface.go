package packageidentity

import (
	"context"
	"fmt"

	"github.com/AjayMunagala/software-engineering-platform/backend/lie/golang"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
)

// Input carries immutable prerequisites for package identity analysis.
type Input struct {
	Snapshot rie.RepositorySnapshot
	Syntax   golang.GoLanguageInventory
}

// Engine deterministically proves Go package identities from local manifests.
type Engine interface {
	Name() string
	Version() string
	ArtifactName() string
	Description() string
	Analyze(context.Context, Input) (GoPackageIdentityInventory, error)
}

// New returns the Phase 2.2.1 package identity engine.
func New(configs ...Config) (Engine, error) {
	config := DefaultConfig()
	if len(configs) > 1 {
		return nil, fmt.Errorf("%w: at most one package identity configuration is accepted", ErrInvalidConfig)
	}
	if len(configs) == 1 {
		config = configs[0].withDefaults()
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return &engine{config: config}, nil
}

// InventoryFrom retrieves the stable artifact from a per-run store.
func InventoryFrom(store *rie.ArtifactStore) (GoPackageIdentityInventory, bool) {
	return rie.ArtifactAs[GoPackageIdentityInventory](store, ArtifactName)
}
