package golang

import (
	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
)

// InventoryFrom retrieves the typed GoLanguageInventory artifact from ArtifactStore.
func InventoryFrom(store *rie.ArtifactStore) (GoLanguageInventory, bool) {
	if store == nil {
		return GoLanguageInventory{}, false
	}
	return rie.ArtifactAs[GoLanguageInventory](store, ArtifactName)
}
