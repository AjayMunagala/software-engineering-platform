// Package metadata implements RIE v0.6 Repository Metadata Engine.
package metadata

import "github.com/AjayMunagala/software-engineering-platform/backend/rie"

type Engine interface{ rie.Engine }

func InventoryFrom(run *rie.RunContext) (RepositoryMetadata, bool) {
	if run == nil {
		return RepositoryMetadata{}, false
	}
	return rie.ArtifactAs[RepositoryMetadata](run.Artifacts, RepositoryMetadataArtifactName)
}
