package summary

import (
	"context"
	"sort"
	"strings"

	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
	metadataengine "github.com/AjayMunagala/software-engineering-platform/backend/rie/metadata"
)

// CompositionEngine creates the immutable repository-intelligence entry point.
type CompositionEngine struct{ config Config }

func New(configs ...Config) Engine {
	config := DefaultConfig()
	if len(configs) > 0 {
		config = configs[0]
	}
	config.UnavailableCapabilities = append([]CapabilityDefinition(nil), config.UnavailableCapabilities...)
	return CompositionEngine{config: config}
}

func (CompositionEngine) Name() string    { return "repository-intelligence-summary" }
func (CompositionEngine) Version() string { return "0.7.0" }
func (CompositionEngine) Description() string {
	return "Composes repository metadata and explicit intelligence availability without duplicating source artifacts"
}

func (engine CompositionEngine) Execute(ctx context.Context, run *rie.RunContext) error {
	if run == nil {
		return rie.ErrRunContextRequired
	}
	metadataInventory, available := metadataengine.InventoryFrom(run)
	if !available || metadataInventory.ArtifactVersion() != metadataengine.RepositoryMetadataArtifactVersion {
		return ErrMetadataRequired
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	capabilities, err := capabilityStatuses(engine.config.UnavailableCapabilities)
	if err != nil {
		return err
	}
	sections := knownSections(metadataInventory)
	inventory := RepositoryIntelligenceSummary{
		metadata:           rie.ArtifactMetadata{Name: RepositoryIntelligenceSummaryArtifactName, Version: RepositoryIntelligenceSummaryArtifactVersion, EngineName: engine.Name(), EngineVersion: engine.Version()},
		repositoryMetadata: metadataInventory, sections: sections, capabilities: capabilities,
	}
	if err := run.Artifacts.Put(inventory); err != nil {
		return err
	}
	run.Report.Summary = reportFromSummary(inventory)
	return nil
}

func knownSections(metadataInventory metadataengine.RepositoryMetadata) []SectionStatus {
	sources := map[string]rie.ArtifactReference{}
	for _, source := range metadataInventory.SourceArtifacts() {
		sources[source.Name] = source
	}
	metadataReference := rie.ArtifactReference{Name: metadataInventory.ArtifactName(), Version: metadataInventory.ArtifactVersion()}
	sections := []SectionStatus{
		{ID: "repository", Status: StatusAvailable, Source: sources["discovery-inventory"]},
		{ID: "layout", Status: StatusAvailable, Source: sources[rie.RepositorySnapshotArtifactName]},
		{ID: "languages", Status: StatusAvailable, Source: sources["language-inventory"]},
		{ID: "frameworks", Status: StatusAvailable, Source: sources["framework-inventory"]},
		{ID: "build", Status: StatusAvailable, Source: sources["build-inventory"]},
		{ID: "workspaces", Status: StatusAvailable, Source: sources["build-inventory"]},
		{ID: "executive-metadata", Status: StatusAvailable, Source: metadataReference},
	}
	sort.Slice(sections, func(i, j int) bool { return sections[i].ID < sections[j].ID })
	return sections
}

func capabilityStatuses(definitions []CapabilityDefinition) ([]CapabilityStatus, error) {
	seen := make(map[string]struct{}, len(definitions))
	items := make([]CapabilityStatus, 0, len(definitions))
	for _, definition := range definitions {
		id := strings.TrimSpace(definition.ID)
		if id == "" {
			return nil, ErrInvalidCapability
		}
		if _, duplicate := seen[id]; duplicate {
			return nil, ErrInvalidCapability
		}
		seen[id] = struct{}{}
		items = append(items, CapabilityStatus{ID: id, Label: definition.Label, Status: StatusUnavailable, Reason: definition.Reason, FutureOwner: definition.FutureOwner})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items, nil
}

func reportFromSummary(summary RepositoryIntelligenceSummary) rie.IntelligenceSummaryReport {
	metadataInventory := summary.RepositoryMetadata()
	report := rie.IntelligenceSummaryReport{
		Artifact:           rie.ArtifactReference{Name: summary.ArtifactName(), Version: summary.ArtifactVersion()},
		RepositoryMetadata: rie.ArtifactReference{Name: metadataInventory.ArtifactName(), Version: metadataInventory.ArtifactVersion()},
		Sections:           []rie.SummarySectionStatus{}, Capabilities: []rie.SummaryCapabilityStatus{},
	}
	for _, section := range summary.Sections() {
		report.Sections = append(report.Sections, rie.SummarySectionStatus{ID: section.ID, Status: section.Status, Source: section.Source})
	}
	for _, capability := range summary.Capabilities() {
		report.Capabilities = append(report.Capabilities, rie.SummaryCapabilityStatus{ID: capability.ID, Label: capability.Label, Status: capability.Status, Reason: capability.Reason, FutureOwner: capability.FutureOwner})
	}
	return report
}
