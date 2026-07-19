package rie

import "time"

// RunContext carries authorized scan input, internal pipeline artifacts, and
// the public report assembled by the registered engines.
type RunContext struct {
	RepositoryPath   string
	Config           Config
	Entries          []RepositoryEntry
	CompletedEngines map[string]string
	Artifacts        *ArtifactStore
	Report           Report
}

// RepositoryEntry is the normalized, repository-relative artifact produced
// by Discovery Engine and consumed by later engines.
type RepositoryEntry struct {
	Path  string
	IsDir bool
}

// Report is the versioned, additive JSON contract for RIE scan results.
type Report struct {
	SchemaVersion string           `json:"schema_version"`
	Scan          ScanMetadata     `json:"scan"`
	Repository    Repository       `json:"repository"`
	Statistics    Statistics       `json:"statistics"`
	Ignore        IgnoreSummary    `json:"ignore"`
	Languages     LanguageSummary  `json:"languages"`
	Frameworks    FrameworkSummary `json:"frameworks"`
	Metrics       Metrics          `json:"metrics"`
	Warnings      []Diagnostic     `json:"warnings"`
	Errors        []Diagnostic     `json:"errors"`
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
