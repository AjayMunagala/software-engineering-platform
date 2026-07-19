package rie

const (
	RepositorySnapshotArtifactName    = "repository-snapshot"
	RepositorySnapshotArtifactVersion = "1.0.0"
)

// ArtifactMetadata identifies a versioned engine artifact.
type ArtifactMetadata struct {
	Name          string
	Version       string
	EngineName    string
	EngineVersion string
}

// RepositorySnapshot is the immutable, ignore-filtered repository artifact.
type RepositorySnapshot struct {
	metadata    ArtifactMetadata
	rootPath    string
	entries     []RepositoryEntry
	statistics  Statistics
	diagnostics []Diagnostic
}

// NewRepositorySnapshot defensively copies canonical repository facts.
func NewRepositorySnapshot(
	rootPath string,
	entries []RepositoryEntry,
	statistics Statistics,
	diagnostics []Diagnostic,
	engineVersion string,
) RepositorySnapshot {
	return RepositorySnapshot{
		metadata: ArtifactMetadata{
			Name: RepositorySnapshotArtifactName, Version: RepositorySnapshotArtifactVersion,
			EngineName: "ignore", EngineVersion: engineVersion,
		},
		rootPath: rootPath, entries: append([]RepositoryEntry(nil), entries...),
		statistics: statistics, diagnostics: append([]Diagnostic(nil), diagnostics...),
	}
}

func (RepositorySnapshot) ArtifactName() string { return RepositorySnapshotArtifactName }

func (RepositorySnapshot) ArtifactVersion() string { return RepositorySnapshotArtifactVersion }

func (snapshot RepositorySnapshot) Metadata() ArtifactMetadata { return snapshot.metadata }

func (snapshot RepositorySnapshot) RootPath() string { return snapshot.rootPath }

// Entries returns a defensive copy of normalized, ignore-filtered entries.
func (snapshot RepositorySnapshot) Entries() []RepositoryEntry {
	return append([]RepositoryEntry(nil), snapshot.entries...)
}

// ForEachEntry visits immutable entry values without copying the entire slice.
// Iteration stops when the visitor returns an error.
func (snapshot RepositorySnapshot) ForEachEntry(visitor func(RepositoryEntry) error) error {
	if visitor == nil {
		return nil
	}
	for _, entry := range snapshot.entries {
		if err := visitor(entry); err != nil {
			return err
		}
	}
	return nil
}

func (snapshot RepositorySnapshot) Statistics() Statistics { return snapshot.statistics }

// Diagnostics returns a defensive copy of diagnostics known when frozen.
func (snapshot RepositorySnapshot) Diagnostics() []Diagnostic {
	return append([]Diagnostic(nil), snapshot.diagnostics...)
}

// RepositorySnapshotFrom retrieves the canonical repository artifact.
func RepositorySnapshotFrom(run *RunContext) (RepositorySnapshot, bool) {
	if run == nil {
		return RepositorySnapshot{}, false
	}
	return ArtifactAs[RepositorySnapshot](run.Artifacts, RepositorySnapshotArtifactName)
}
