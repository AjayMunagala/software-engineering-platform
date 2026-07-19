// Package build implements RIE v0.5 Build & Package Intelligence Engine.
package build

import (
	"context"

	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
)

// Engine is the public contract for deterministic build and package detection.
type Engine interface {
	rie.Engine
}

// Candidate is one supported repository file presented to a registry detector.
type Candidate struct {
	Path    string
	Content []byte
}

// Detector is one in-code registry entry. Implementations are read-only.
type Detector interface {
	ID() string
	FileNames() []string
	RequiresContent() bool
	Detect(context.Context, Candidate) ([]Finding, error)
}

// InventoryFrom retrieves the typed BuildInventory artifact for later engines.
func InventoryFrom(run *rie.RunContext) (BuildInventory, bool) {
	if run == nil {
		return BuildInventory{}, false
	}
	return rie.ArtifactAs[BuildInventory](run.Artifacts, BuildInventoryArtifactName)
}
