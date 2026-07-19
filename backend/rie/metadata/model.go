package metadata

import "github.com/AjayMunagala/software-engineering-platform/backend/rie"

const (
	RepositoryMetadataArtifactName    = "repository-metadata"
	RepositoryMetadataArtifactVersion = "1.0.0"
)

type RepositoryIdentity struct {
	Name     string
	RootPath string
	Git      GitInformation
}

type GitInformation struct {
	Present       bool
	CurrentBranch string
	DefaultBranch string
}

type Layout struct {
	topLevelDirectories []string
	topLevelFiles       []string
	MaximumDepth        int
}

func (layout Layout) TopLevelDirectories() []string {
	return append([]string(nil), layout.topLevelDirectories...)
}
func (layout Layout) TopLevelFiles() []string { return append([]string(nil), layout.topLevelFiles...) }

type Language struct {
	Name       string
	FileCount  int
	Percentage float64
}

type Framework struct {
	Name      string
	Ecosystem string
	locations []string
}

func (item Framework) Locations() []string { return append([]string(nil), item.locations...) }

type Technology struct {
	ID        string
	Name      string
	locations []string
}

func (item Technology) Locations() []string { return append([]string(nil), item.locations...) }

type Toolchain struct {
	Tool        string
	constraints []string
	locations   []string
}

func (item Toolchain) Constraints() []string { return append([]string(nil), item.constraints...) }
func (item Toolchain) Locations() []string   { return append([]string(nil), item.locations...) }

type BuildOverview struct {
	packageManagers []Technology
	buildSystems    []Technology
	toolchains      []Toolchain
	LockFileCount   int
}

func (overview BuildOverview) PackageManagers() []Technology {
	return copyTechnologies(overview.packageManagers)
}
func (overview BuildOverview) BuildSystems() []Technology {
	return copyTechnologies(overview.buildSystems)
}
func (overview BuildOverview) Toolchains() []Toolchain { return copyToolchains(overview.toolchains) }

// RepositoryMetadata is the immutable executive summary artifact.
type RepositoryMetadata struct {
	metadata            rie.ArtifactMetadata
	repository          RepositoryIdentity
	statistics          rie.Statistics
	layout              Layout
	monorepo            bool
	workspaceCount      int
	declaredModuleCount int
	languages           []Language
	frameworks          []Framework
	build               BuildOverview
	sources             []rie.ArtifactReference
}

func (RepositoryMetadata) ArtifactName() string                     { return RepositoryMetadataArtifactName }
func (RepositoryMetadata) ArtifactVersion() string                  { return RepositoryMetadataArtifactVersion }
func (inventory RepositoryMetadata) Metadata() rie.ArtifactMetadata { return inventory.metadata }
func (inventory RepositoryMetadata) Repository() RepositoryIdentity { return inventory.repository }
func (inventory RepositoryMetadata) Statistics() rie.Statistics     { return inventory.statistics }
func (inventory RepositoryMetadata) Layout() Layout                 { return copyLayout(inventory.layout) }
func (inventory RepositoryMetadata) Monorepo() bool                 { return inventory.monorepo }
func (inventory RepositoryMetadata) WorkspaceCount() int            { return inventory.workspaceCount }
func (inventory RepositoryMetadata) DeclaredModuleCount() int       { return inventory.declaredModuleCount }
func (inventory RepositoryMetadata) Languages() []Language {
	return append([]Language(nil), inventory.languages...)
}
func (inventory RepositoryMetadata) Frameworks() []Framework {
	return copyFrameworks(inventory.frameworks)
}
func (inventory RepositoryMetadata) Build() BuildOverview { return copyBuildOverview(inventory.build) }
func (inventory RepositoryMetadata) SourceArtifacts() []rie.ArtifactReference {
	return append([]rie.ArtifactReference(nil), inventory.sources...)
}

func copyLayout(layout Layout) Layout {
	layout.topLevelDirectories = append([]string(nil), layout.topLevelDirectories...)
	layout.topLevelFiles = append([]string(nil), layout.topLevelFiles...)
	return layout
}

func copyFrameworks(items []Framework) []Framework {
	result := append([]Framework(nil), items...)
	for index := range result {
		result[index].locations = append([]string(nil), result[index].locations...)
	}
	return result
}

func copyTechnologies(items []Technology) []Technology {
	result := append([]Technology(nil), items...)
	for index := range result {
		result[index].locations = append([]string(nil), result[index].locations...)
	}
	return result
}

func copyToolchains(items []Toolchain) []Toolchain {
	result := append([]Toolchain(nil), items...)
	for index := range result {
		result[index].constraints = append([]string(nil), result[index].constraints...)
		result[index].locations = append([]string(nil), result[index].locations...)
	}
	return result
}

func copyBuildOverview(overview BuildOverview) BuildOverview {
	overview.packageManagers = copyTechnologies(overview.packageManagers)
	overview.buildSystems = copyTechnologies(overview.buildSystems)
	overview.toolchains = copyToolchains(overview.toolchains)
	return overview
}
