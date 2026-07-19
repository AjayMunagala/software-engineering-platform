package summary

import (
	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
	metadataengine "github.com/AjayMunagala/software-engineering-platform/backend/rie/metadata"
)

const (
	RepositoryIntelligenceSummaryArtifactName    = "repository-intelligence-summary"
	RepositoryIntelligenceSummaryArtifactVersion = "1.0.0"
	StatusAvailable                              = "available"
	StatusUnavailable                            = "unavailable"
)

type SectionStatus struct {
	ID     string
	Status string
	Source rie.ArtifactReference
}

type CapabilityStatus struct {
	ID          string
	Label       string
	Status      string
	Reason      string
	FutureOwner string
}

// RepositoryIntelligenceSummary composes metadata without duplicating its views.
type RepositoryIntelligenceSummary struct {
	metadata           rie.ArtifactMetadata
	repositoryMetadata metadataengine.RepositoryMetadata
	sections           []SectionStatus
	capabilities       []CapabilityStatus
}

func (RepositoryIntelligenceSummary) ArtifactName() string {
	return RepositoryIntelligenceSummaryArtifactName
}
func (RepositoryIntelligenceSummary) ArtifactVersion() string {
	return RepositoryIntelligenceSummaryArtifactVersion
}
func (summary RepositoryIntelligenceSummary) Metadata() rie.ArtifactMetadata { return summary.metadata }
func (summary RepositoryIntelligenceSummary) RepositoryMetadata() metadataengine.RepositoryMetadata {
	return summary.repositoryMetadata
}
func (summary RepositoryIntelligenceSummary) Sections() []SectionStatus {
	return append([]SectionStatus(nil), summary.sections...)
}
func (summary RepositoryIntelligenceSummary) Capabilities() []CapabilityStatus {
	return append([]CapabilityStatus(nil), summary.capabilities...)
}
