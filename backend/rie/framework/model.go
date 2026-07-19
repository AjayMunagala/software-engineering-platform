package framework

import "github.com/AjayMunagala/software-engineering-platform/backend/rie"

const (
	FrameworkInventoryArtifactName    = "framework-inventory"
	FrameworkInventoryArtifactVersion = "1.0.0"
)

// Metadata identifies the artifact and its producing engine.
type Metadata struct {
	Name          string
	Version       string
	EngineName    string
	EngineVersion string
}

// FrameworkItem is one evidence-backed framework detection.
type FrameworkItem struct {
	Name      string
	Ecosystem string
	evidence  []rie.Evidence
}

// Evidence returns a defensive copy of location-aware detection evidence.
func (item FrameworkItem) Evidence() []rie.Evidence {
	return append([]rie.Evidence(nil), item.evidence...)
}

// FrameworkSummary contains aggregate inventory counts.
type FrameworkSummary struct {
	ManifestsInspected int
}

// FrameworkInventory is the immutable artifact consumed by later engines.
type FrameworkInventory struct {
	metadata Metadata
	items    []FrameworkItem
	summary  FrameworkSummary
}

func newFrameworkInventory(items []FrameworkItem, summary FrameworkSummary) FrameworkInventory {
	copied := make([]FrameworkItem, len(items))
	for index, item := range items {
		copied[index] = item
		copied[index].evidence = append([]rie.Evidence(nil), item.evidence...)
	}
	return FrameworkInventory{
		metadata: Metadata{
			Name: FrameworkInventoryArtifactName, Version: FrameworkInventoryArtifactVersion,
			EngineName: "framework", EngineVersion: "0.4.2",
		},
		items: copied, summary: summary,
	}
}

func (FrameworkInventory) ArtifactName() string { return FrameworkInventoryArtifactName }

func (FrameworkInventory) ArtifactVersion() string { return FrameworkInventoryArtifactVersion }

func (inventory FrameworkInventory) Metadata() Metadata { return inventory.metadata }

// Items returns a deep defensive copy.
func (inventory FrameworkInventory) Items() []FrameworkItem {
	items := make([]FrameworkItem, len(inventory.items))
	for index, item := range inventory.items {
		items[index] = item
		items[index].evidence = append([]rie.Evidence(nil), item.evidence...)
	}
	return items
}

func (inventory FrameworkInventory) Summary() FrameworkSummary { return inventory.summary }

type detection struct {
	name      string
	ecosystem string
	rule      string
	value     string
}
