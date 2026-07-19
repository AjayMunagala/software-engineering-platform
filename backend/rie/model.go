package rie

import "time"

// RunContext carries authorized scan input, internal pipeline artifacts, and
// the public report assembled by the registered engines.
type RunContext struct {
	RepositoryPath string
	Config         Config
	Entries        []RepositoryEntry
	Report         Report
}

// RepositoryEntry is the normalized, repository-relative artifact produced
// by Discovery Engine and consumed by later engines.
type RepositoryEntry struct {
	Path  string
	IsDir bool
}

// Report is the versioned, additive JSON contract for RIE scan results.
type Report struct {
	SchemaVersion string        `json:"schema_version"`
	Scan          ScanMetadata  `json:"scan"`
	Repository    Repository    `json:"repository"`
	Statistics    Statistics    `json:"statistics"`
	Ignore        IgnoreSummary `json:"ignore"`
	Metrics       Metrics       `json:"metrics"`
	Warnings      []Diagnostic  `json:"warnings"`
	Errors        []Diagnostic  `json:"errors"`
}

// ScanMetadata identifies one pipeline run and records its timing.
type ScanMetadata struct {
	ID                   string    `json:"id"`
	StartedAt            time.Time `json:"started_at"`
	FinishedAt           time.Time `json:"finished_at"`
	DurationMilliseconds float64   `json:"duration_ms"`
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
