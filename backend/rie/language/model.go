package language

const (
	// LanguageInventoryArtifactName is the stable artifact-store key.
	LanguageInventoryArtifactName = "language-inventory"
	// LanguageInventoryArtifactVersion freezes the v0.3 consumer contract.
	LanguageInventoryArtifactVersion = "1.0.0"
)

// Metadata identifies the immutable artifact and its producing engine.
type Metadata struct {
	Name          string
	Version       string
	EngineName    string
	EngineVersion string
}

// LanguageItem is the stable, engine-facing language detection record.
type LanguageItem struct {
	Name       string
	Count      int
	Percentage float64
}

// LanguageSummary contains aggregate inventory counts.
type LanguageSummary struct {
	DetectedFiles int
	UnknownFiles  int
}

// LanguageInventory is the immutable artifact consumed by future engines.
// Its slices are private and accessors return defensive copies.
type LanguageInventory struct {
	metadata Metadata
	items    []LanguageItem
	summary  LanguageSummary
}

func newLanguageInventory(items []LanguageItem, summary LanguageSummary) LanguageInventory {
	return LanguageInventory{
		metadata: Metadata{
			Name: LanguageInventoryArtifactName, Version: LanguageInventoryArtifactVersion,
			EngineName: "language", EngineVersion: "0.3.2",
		},
		items:   append([]LanguageItem(nil), items...),
		summary: summary,
	}
}

func (LanguageInventory) ArtifactName() string { return LanguageInventoryArtifactName }

func (LanguageInventory) ArtifactVersion() string { return LanguageInventoryArtifactVersion }

// Metadata returns immutable artifact identity by value.
func (inventory LanguageInventory) Metadata() Metadata { return inventory.metadata }

// Items returns a defensive copy that consumers may modify safely.
func (inventory LanguageInventory) Items() []LanguageItem {
	return append([]LanguageItem(nil), inventory.items...)
}

// Summary returns aggregate counts by value.
func (inventory LanguageInventory) Summary() LanguageSummary { return inventory.summary }
