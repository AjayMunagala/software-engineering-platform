// Package language implements RIE v0.3 Language Engine.
package language

import "github.com/AjayMunagala/software-engineering-platform/backend/rie"

// Engine is the public contract for deterministic language detection.
type Engine interface {
	rie.Engine
}

// InventoryFrom retrieves the stable LanguageInventory artifact for later engines.
func InventoryFrom(run *rie.RunContext) (LanguageInventory, bool) {
	if run == nil {
		return LanguageInventory{}, false
	}
	return rie.ArtifactAs[LanguageInventory](run.Artifacts, LanguageInventoryArtifactName)
}
