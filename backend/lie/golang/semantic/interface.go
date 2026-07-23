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

// InventoryFrom retrieves a semantic artifact from a per-run artifact store.
func InventoryFrom(store *rie.ArtifactStore) (GoSemanticInventory, bool) {
	return rie.ArtifactAs[GoSemanticInventory](store, ArtifactName)
}
