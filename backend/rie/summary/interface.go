// Package summary implements RIE v0.7 Repository Intelligence Summary.
package summary

import "github.com/AjayMunagala/software-engineering-platform/backend/rie"

type Engine interface{ rie.Engine }

func InventoryFrom(run *rie.RunContext) (RepositoryIntelligenceSummary, bool) {
	if run == nil {
		return RepositoryIntelligenceSummary{}, false
	}
	return rie.ArtifactAs[RepositoryIntelligenceSummary](run.Artifacts, RepositoryIntelligenceSummaryArtifactName)
}
