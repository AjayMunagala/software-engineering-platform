package rie

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

const SchemaVersion = "0.5.0"

// Pipeline executes an ordered, configurable sequence of RIE engines.
type Pipeline struct {
	engines []Engine
}

// New creates an empty RIE pipeline. Engines are added through Register.
func New() *Pipeline {
	return &Pipeline{engines: []Engine{}}
}

// Register appends one engine without changing pipeline orchestration logic.
func (pipeline *Pipeline) Register(engine Engine) error {
	if engine == nil {
		return ErrNilEngine
	}
	for _, registered := range pipeline.engines {
		if registered.Name() == engine.Name() {
			return fmt.Errorf("engine %q is already registered", engine.Name())
		}
	}
	pipeline.engines = append(pipeline.engines, engine)
	return nil
}

// Engines returns a copy of the registered engine sequence.
func (pipeline *Pipeline) Engines() []Engine {
	return append([]Engine(nil), pipeline.engines...)
}

// Run executes each registered engine in order and stops at the first error.
func (pipeline *Pipeline) Run(ctx context.Context, run *RunContext) (err error) {
	if run == nil {
		return ErrRunContextRequired
	}

	startedAt := time.Now()
	run.Report = Report{
		SchemaVersion: SchemaVersion,
		Scan: ScanMetadata{
			ID:        newScanID(startedAt),
			StartedAt: startedAt.UTC(),
			Engines:   []EngineMetadata{},
		},
		Warnings: []Diagnostic{},
		Errors:   []Diagnostic{},
		Ignore: IgnoreSummary{
			Sources: []string{},
		},
		Languages: LanguageSummary{
			Items: []Language{},
		},
		Frameworks: FrameworkSummary{
			Items: []Framework{},
		},
		Build: BuildSummary{
			PackageManagers: []BuildTool{}, BuildSystems: []BuildTool{},
			Workspaces: []BuildWorkspace{}, LockFiles: []BuildLockFile{},
			Toolchains: []BuildToolchain{},
		},
	}
	if run.CompletedEngines == nil {
		run.CompletedEngines = make(map[string]string)
	} else {
		clear(run.CompletedEngines)
	}
	if run.Artifacts == nil {
		run.Artifacts = NewArtifactStore()
	} else {
		run.Artifacts.Reset()
	}
	defer func() {
		finishedAt := time.Now()
		duration := finishedAt.Sub(startedAt)
		run.Report.Scan.FinishedAt = finishedAt.UTC()
		run.Report.Scan.DurationMilliseconds = float64(duration) / float64(time.Millisecond)
		if duration > 0 {
			run.Report.Metrics.FilesPerSecond = float64(run.Report.Statistics.Files) / duration.Seconds()
		}
	}()

	if configErr := run.Config.Validate(); configErr != nil {
		run.Report.Errors = append(run.Report.Errors, Diagnostic{
			Engine:  "pipeline",
			Code:    "invalid_configuration",
			Message: configErr.Error(),
		})
		return configErr
	}

	for _, engine := range pipeline.engines {
		if err = engine.Execute(ctx, run); err != nil {
			run.Report.Errors = append(run.Report.Errors, Diagnostic{
				Engine:  engine.Name(),
				Code:    "execution_failed",
				Message: err.Error(),
			})
			return fmt.Errorf("execute %s %s: %w", engine.Name(), engine.Version(), err)
		}
		run.CompletedEngines[engine.Name()] = engine.Version()
		run.Report.Scan.Engines = append(run.Report.Scan.Engines, EngineMetadata{
			Name: engine.Name(), Version: engine.Version(), Description: engine.Description(),
		})
	}

	return nil
}

func newScanID(startedAt time.Time) string {
	var random [8]byte
	if _, err := rand.Read(random[:]); err == nil {
		return hex.EncodeToString(random[:])
	}
	return fmt.Sprintf("scan-%d", startedAt.UnixNano())
}
