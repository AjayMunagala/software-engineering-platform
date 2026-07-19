package rie_test

import (
	"testing"

	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
	buildengine "github.com/AjayMunagala/software-engineering-platform/backend/rie/build"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie/discovery"
	frameworkengine "github.com/AjayMunagala/software-engineering-platform/backend/rie/framework"
	languageengine "github.com/AjayMunagala/software-engineering-platform/backend/rie/language"
	metadataengine "github.com/AjayMunagala/software-engineering-platform/backend/rie/metadata"
	summaryengine "github.com/AjayMunagala/software-engineering-platform/backend/rie/summary"
)

func TestRIEV1ContractVersions(t *testing.T) {
	t.Parallel()
	versions := map[string]string{
		"rie":       rie.Version,
		"schema":    rie.SchemaVersion,
		"discovery": discovery.DiscoveryInventoryArtifactVersion,
		"snapshot":  rie.RepositorySnapshotArtifactVersion,
		"language":  languageengine.LanguageInventoryArtifactVersion,
		"framework": frameworkengine.FrameworkInventoryArtifactVersion,
		"build":     buildengine.BuildInventoryArtifactVersion,
		"metadata":  metadataengine.RepositoryMetadataArtifactVersion,
		"summary":   summaryengine.RepositoryIntelligenceSummaryArtifactVersion,
	}
	for name, version := range versions {
		if version != "1.0.0" {
			t.Errorf("%s version = %q, want 1.0.0", name, version)
		}
	}
}
