package discovery

import "github.com/AjayMunagala/software-engineering-platform/backend/rie"

// Report aliases the shared, versioned RIE report contract.
type Report = rie.Report

// Entry aliases the normalized artifact passed to later engines.
type Entry = rie.RepositoryEntry

const (
	DiscoveryInventoryArtifactName    = "discovery-inventory"
	DiscoveryInventoryArtifactVersion = "1.0.0"
)

// RepositoryIdentity contains deterministic identity facts discovered locally.
type RepositoryIdentity struct {
	Name          string
	RootPath      string
	Git           bool
	CurrentBranch string
	DefaultBranch string
}

// DiscoveryInventory is the immutable repository identity artifact.
type DiscoveryInventory struct {
	metadata   rie.ArtifactMetadata
	repository RepositoryIdentity
	statistics rie.Statistics
}

func newDiscoveryInventory(repository RepositoryIdentity, statistics rie.Statistics) DiscoveryInventory {
	return DiscoveryInventory{
		metadata: rie.ArtifactMetadata{
			Name: DiscoveryInventoryArtifactName, Version: DiscoveryInventoryArtifactVersion,
			EngineName: "discovery", EngineVersion: "0.1.1",
		},
		repository: repository, statistics: statistics,
	}
}

func (DiscoveryInventory) ArtifactName() string { return DiscoveryInventoryArtifactName }

func (DiscoveryInventory) ArtifactVersion() string { return DiscoveryInventoryArtifactVersion }

func (inventory DiscoveryInventory) Metadata() rie.ArtifactMetadata { return inventory.metadata }

func (inventory DiscoveryInventory) Repository() RepositoryIdentity { return inventory.repository }

func (inventory DiscoveryInventory) Statistics() rie.Statistics { return inventory.statistics }

// InventoryFrom retrieves the typed DiscoveryInventory artifact.
func InventoryFrom(run *rie.RunContext) (DiscoveryInventory, bool) {
	if run == nil {
		return DiscoveryInventory{}, false
	}
	return rie.ArtifactAs[DiscoveryInventory](run.Artifacts, DiscoveryInventoryArtifactName)
}
