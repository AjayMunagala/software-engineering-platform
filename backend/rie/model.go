package rie

import "time"

// RunContext carries authorized scan input, internal pipeline artifacts, and
// the public report assembled by the registered engines.
type RunContext struct {
	RepositoryPath string
	Config         Config
	// Entries is the internal Discovery-to-Ignore compatibility bridge.
	// New repository-level engines must consume RepositorySnapshot instead.
	Entries   []RepositoryEntry
	Artifacts *ArtifactStore
	Report    Report
}

// RepositoryEntry is the normalized, repository-relative artifact produced
// by Discovery Engine and consumed by later engines.
type RepositoryEntry struct {
	Path  string
	IsDir bool
}

// Report is the versioned, additive JSON contract for RIE scan results.
type Report struct {
	SchemaVersion string                    `json:"schema_version"`
	Scan          ScanMetadata              `json:"scan"`
	Repository    Repository                `json:"repository"`
	Statistics    Statistics                `json:"statistics"`
	Ignore        IgnoreSummary             `json:"ignore"`
	Languages     LanguageSummary           `json:"languages"`
	Frameworks    FrameworkSummary          `json:"frameworks"`
	Build         BuildSummary              `json:"build"`
	Metadata      RepositoryMetadataSummary `json:"metadata"`
	Summary       IntelligenceSummaryReport `json:"summary"`
	Metrics       Metrics                   `json:"metrics"`
	Warnings      []Diagnostic              `json:"warnings"`
	Errors        []Diagnostic              `json:"errors"`
}

// ScanMetadata identifies one pipeline run and records its timing.
type ScanMetadata struct {
	ID                   string           `json:"id"`
	StartedAt            time.Time        `json:"started_at"`
	FinishedAt           time.Time        `json:"finished_at"`
	DurationMilliseconds float64          `json:"duration_ms"`
	Engines              []EngineMetadata `json:"engines"`
}

// EngineMetadata describes an engine that completed during a scan.
type EngineMetadata struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
}

// Repository identifies the inspected local repository.
type Repository struct {
	Name     string `json:"name"`
	RootPath string `json:"root_path"`
	Git      bool   `json:"git"`
}

// Statistics contains deterministic repository counts after ignore rules.
type Statistics struct {
	Files   int `json:"files"`
	Folders int `json:"folders"`
}

// IgnoreSummary describes the rules loaded and entries excluded by Ignore Engine.
type IgnoreSummary struct {
	Rules          int      `json:"rules"`
	IgnoredFiles   int      `json:"ignored_files"`
	IgnoredFolders int      `json:"ignored_folders"`
	Sources        []string `json:"sources"`
}

// LanguageSummary contains deterministic extension-based language detection.
type LanguageSummary struct {
	DetectedFiles int        `json:"detected_files"`
	UnknownFiles  int        `json:"unknown_files"`
	Items         []Language `json:"items"`
}

// Language records the detected file count and share of recognized files.
type Language struct {
	Name       string  `json:"name"`
	FileCount  int     `json:"file_count"`
	Percentage float64 `json:"percentage"`
}

// FrameworkSummary is the presentation-oriented framework report.
type FrameworkSummary struct {
	ManifestsInspected int         `json:"manifests_inspected"`
	Items              []Framework `json:"items"`
}

// Framework records one deterministic detection and its manifest evidence.
type Framework struct {
	Name      string     `json:"name"`
	Ecosystem string     `json:"ecosystem"`
	Evidence  []Evidence `json:"evidence"`
}

// Evidence records the source and deterministic rule supporting a detection.
// File is repository-relative so it also preserves the detected project location.
type Evidence struct {
	File  string `json:"file"`
	Rule  string `json:"rule"`
	Value string `json:"value"`
}

// BuildSummary is the presentation-oriented BuildInventory projection.
type BuildSummary struct {
	PackageManagers []BuildTool      `json:"package_managers"`
	BuildSystems    []BuildTool      `json:"build_systems"`
	Workspaces      []BuildWorkspace `json:"workspaces"`
	LockFiles       []BuildLockFile  `json:"lock_files"`
	Toolchains      []BuildToolchain `json:"toolchains"`
}

type BuildTool struct {
	ID       string     `json:"id"`
	Name     string     `json:"name"`
	Location string     `json:"location"`
	Evidence []Evidence `json:"evidence"`
}

type BuildWorkspace struct {
	ID       string     `json:"id"`
	Kind     string     `json:"kind"`
	Location string     `json:"location"`
	Members  []string   `json:"members"`
	Evidence []Evidence `json:"evidence"`
}

type BuildLockFile struct {
	PackageManagerID string     `json:"package_manager_id"`
	Path             string     `json:"path"`
	Location         string     `json:"location"`
	Evidence         []Evidence `json:"evidence"`
}

type BuildToolchain struct {
	Tool       string     `json:"tool"`
	Constraint string     `json:"constraint"`
	Location   string     `json:"location"`
	Evidence   []Evidence `json:"evidence"`
}

// RepositoryMetadataSummary is the executive repository cover page.
type RepositoryMetadataSummary struct {
	Name                string               `json:"name"`
	RootPath            string               `json:"root_path"`
	Git                 GitMetadata          `json:"git"`
	Statistics          Statistics           `json:"statistics"`
	Layout              RepositoryLayout     `json:"layout"`
	Monorepo            bool                 `json:"monorepo"`
	WorkspaceCount      int                  `json:"workspace_count"`
	DeclaredModuleCount int                  `json:"declared_module_count"`
	Languages           []MetadataLanguage   `json:"languages"`
	Frameworks          []MetadataFramework  `json:"frameworks"`
	Build               MetadataBuildSummary `json:"build"`
	SourceArtifacts     []ArtifactReference  `json:"source_artifacts"`
}

type GitMetadata struct {
	Present       bool   `json:"present"`
	CurrentBranch string `json:"current_branch,omitempty"`
	DefaultBranch string `json:"default_branch,omitempty"`
}

type RepositoryLayout struct {
	TopLevelDirectories []string `json:"top_level_directories"`
	TopLevelFiles       []string `json:"top_level_files"`
	MaximumDepth        int      `json:"maximum_depth"`
}

type MetadataLanguage struct {
	Name       string  `json:"name"`
	FileCount  int     `json:"file_count"`
	Percentage float64 `json:"percentage"`
}

type MetadataFramework struct {
	Name      string   `json:"name"`
	Ecosystem string   `json:"ecosystem"`
	Locations []string `json:"locations"`
}

type MetadataTechnology struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Locations []string `json:"locations"`
}

type MetadataToolchain struct {
	Tool        string   `json:"tool"`
	Constraints []string `json:"constraints"`
	Locations   []string `json:"locations"`
}

type MetadataBuildSummary struct {
	PackageManagers []MetadataTechnology `json:"package_managers"`
	BuildSystems    []MetadataTechnology `json:"build_systems"`
	Toolchains      []MetadataToolchain  `json:"toolchains"`
	LockFileCount   int                  `json:"lock_file_count"`
}

type ArtifactReference struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// IntelligenceSummaryReport indexes repository intelligence without repeating it.
type IntelligenceSummaryReport struct {
	Artifact           ArtifactReference         `json:"artifact"`
	RepositoryMetadata ArtifactReference         `json:"repository_metadata"`
	Sections           []SummarySectionStatus    `json:"sections"`
	Capabilities       []SummaryCapabilityStatus `json:"capabilities"`
}

type SummarySectionStatus struct {
	ID     string            `json:"id"`
	Status string            `json:"status"`
	Source ArtifactReference `json:"source"`
}

type SummaryCapabilityStatus struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Status      string `json:"status"`
	Reason      string `json:"reason"`
	FutureOwner string `json:"future_owner"`
}

// Metrics records measured scan behavior. Future engines can add metrics
// without changing the repository or statistics contracts.
type Metrics struct {
	FilesPerSecond float64 `json:"files_per_second"`
}

// Diagnostic is a standardized warning or error emitted by an engine.
type Diagnostic struct {
	Engine  string `json:"engine"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
}
