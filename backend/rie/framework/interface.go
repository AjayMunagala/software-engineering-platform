// Package framework implements RIE v0.4 Framework Engine.
package framework

import "github.com/AjayMunagala/software-engineering-platform/backend/rie"

// Engine is the public contract for deterministic framework detection.
type Engine interface {
	rie.Engine
}

// InventoryFrom retrieves the typed FrameworkInventory artifact for later engines.
func InventoryFrom(run *rie.RunContext) (FrameworkInventory, bool) {
	if run == nil {
		return FrameworkInventory{}, false
	}
	return rie.ArtifactAs[FrameworkInventory](run.Artifacts, FrameworkInventoryArtifactName)
}
