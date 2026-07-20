package lie

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie/language"
)

// Runner manages the registration and execution of LIE language engines.
type Runner struct {
	engines []Engine
}

// New creates a LIE runner and registers engines in execution order.
func New(engines ...Engine) (*Runner, error) {
	runner := &Runner{engines: []Engine{}}
	for _, engine := range engines {
		if err := runner.Register(engine); err != nil {
			return nil, err
		}
	}
	return runner, nil
}

// Register adds a language engine to the runner.
func (r *Runner) Register(engine Engine) error {
	if engine == nil {
		return ErrNilEngine
	}
	name := strings.TrimSpace(engine.Name())
	lang := strings.TrimSpace(engine.Language())
	artName := strings.TrimSpace(engine.ArtifactName())
	if name == "" || strings.TrimSpace(engine.Version()) == "" || lang == "" || artName == "" || strings.TrimSpace(engine.Description()) == "" {
		return ErrInvalidEngineMetadata
	}

	for _, existing := range r.engines {
		if strings.EqualFold(existing.Name(), name) {
			return ErrDuplicateEngine
		}
		if strings.EqualFold(existing.Language(), lang) {
			return ErrDuplicateEngine
		}
		if strings.EqualFold(existing.ArtifactName(), artName) {
			return ErrDuplicateArtifactName
		}
	}

	r.engines = append(r.engines, engine)
	return nil
}

// Engines returns a copy of registered engine metadata.
func (r *Runner) Engines() []EngineMetadata {
	meta := make([]EngineMetadata, 0, len(r.engines))
	for _, e := range r.engines {
		meta = append(meta, EngineMetadata{
			Name:         e.Name(),
			Version:      e.Version(),
			Language:     e.Language(),
			ArtifactName: e.ArtifactName(),
			Description:  e.Description(),
		})
	}
	return meta
}

// Run executes all registered language engines against the prerequisite artifacts in store.
func (r *Runner) Run(ctx context.Context, store *rie.ArtifactStore) (RunReport, error) {
	startedAt := time.Now()
	if ctx == nil {
		return RunReport{}, ErrContextRequired
	}
	if store == nil {
		return RunReport{}, ErrArtifactStoreRequired
	}

	snapshotArtifact, ok := store.Get(rie.RepositorySnapshotArtifactName)
	if !ok {
		return RunReport{}, ErrSnapshotRequired
	}
	if snapshotArtifact.ArtifactVersion() != requiredSnapshotVersion {
		return RunReport{}, fmt.Errorf("%w: %s requires %s, got %s", ErrArtifactVersionMismatch, rie.RepositorySnapshotArtifactName, requiredSnapshotVersion, snapshotArtifact.ArtifactVersion())
	}
	snapshot, ok := snapshotArtifact.(rie.RepositorySnapshot)
	if !ok {
		return RunReport{}, ErrSnapshotRequired
	}

	languageArtifact, ok := store.Get(language.LanguageInventoryArtifactName)
	if !ok {
		return RunReport{}, ErrLanguageInventoryRequired
	}
	if languageArtifact.ArtifactVersion() != requiredLanguageVersion {
		return RunReport{}, fmt.Errorf("%w: %s requires %s, got %s", ErrArtifactVersionMismatch, language.LanguageInventoryArtifactName, requiredLanguageVersion, languageArtifact.ArtifactVersion())
	}
	languages, ok := languageArtifact.(language.LanguageInventory)
	if !ok {
		return RunReport{}, ErrLanguageInventoryRequired
	}

	for _, engine := range r.engines {
		if _, exists := store.Get(engine.ArtifactName()); exists {
			return RunReport{}, fmt.Errorf("%w: %s", rie.ErrArtifactAlreadyExists, engine.ArtifactName())
		}
	}

	report := newRunReport(startedAt)
	input := Input{
		Snapshot:  snapshot,
		Languages: languages,
	}

	for _, engine := range r.engines {
		if err := ctx.Err(); err != nil {
			report.finishedAt = time.Now()
			return report, err
		}

		artifact, err := engine.Analyze(ctx, input)
		if err != nil {
			report.finishedAt = time.Now()
			report.fatalDiagnostics = append(report.fatalDiagnostics, Diagnostic{
				Engine:   engine.Name(),
				Severity: SeverityError,
				Code:     "engine_analysis_failed",
				Message:  err.Error(),
			})
			return report, err
		}

		if artifact == nil {
			report.finishedAt = time.Now()
			return report, fmt.Errorf("%w: %s", ErrArtifactRequired, engine.Name())
		}
		if artifact.ArtifactName() != engine.ArtifactName() || !strings.EqualFold(artifact.Language(), engine.Language()) || strings.TrimSpace(artifact.ArtifactVersion()) == "" {
			report.finishedAt = time.Now()
			return report, fmt.Errorf("%w: engine=%s artifact=%s language=%s", ErrArtifactContractMismatch, engine.Name(), artifact.ArtifactName(), artifact.Language())
		}
		if err := store.Put(artifact); err != nil {
			report.finishedAt = time.Now()
			return report, err
		}
		report.engines = append(report.engines, metadataFor(engine))
		report.published = append(report.published, rie.ArtifactReference{Name: artifact.ArtifactName(), Version: artifact.ArtifactVersion()})
	}

	report.finishedAt = time.Now()
	return report, nil
}

func metadataFor(engine Engine) EngineMetadata {
	return EngineMetadata{Name: engine.Name(), Version: engine.Version(), Language: engine.Language(), ArtifactName: engine.ArtifactName(), Description: engine.Description()}
}
