package summary

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
	buildengine "github.com/AjayMunagala/software-engineering-platform/backend/rie/build"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie/discovery"
	frameworkengine "github.com/AjayMunagala/software-engineering-platform/backend/rie/framework"
	ignoreengine "github.com/AjayMunagala/software-engineering-platform/backend/rie/ignore"
	languageengine "github.com/AjayMunagala/software-engineering-platform/backend/rie/language"
	metadataengine "github.com/AjayMunagala/software-engineering-platform/backend/rie/metadata"
)

func TestSummaryComposesMetadataWithoutDuplicatingViews(t *testing.T) {
	t.Parallel()
	repository := t.TempDir()
	mustWrite(t, filepath.Join(repository, "go.mod"), "module example\ngo 1.24\n")
	mustWrite(t, filepath.Join(repository, "main.go"), "package main")
	run := rie.NewRunContext(repository, rie.DefaultConfig())
	if err := fullPipeline(t, true).Run(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	summary, exists := InventoryFrom(run)
	if !exists {
		t.Fatal("RepositoryIntelligenceSummary was not published")
	}
	composed := summary.RepositoryMetadata()
	if composed.ArtifactName() != metadataengine.RepositoryMetadataArtifactName || composed.Repository().Name != filepath.Base(repository) {
		t.Errorf("RepositoryMetadata = %#v", composed.Repository())
	}
	if len(summary.Sections()) != 7 {
		t.Errorf("Sections = %#v", summary.Sections())
	}
	for _, section := range summary.Sections() {
		if section.Status != StatusAvailable || section.Source.Name == "" || section.Source.Version == "" {
			t.Errorf("Section = %#v", section)
		}
	}
	if run.Report.Summary.RepositoryMetadata.Name != metadataengine.RepositoryMetadataArtifactName || run.Report.Summary.Artifact.Name != RepositoryIntelligenceSummaryArtifactName {
		t.Errorf("Report.Summary = %#v", run.Report.Summary)
	}
	if summary.ArtifactVersion() != "1.0.0" || summary.Metadata().EngineVersion != "0.7.0" {
		t.Errorf("Metadata = %#v", summary.Metadata())
	}
}

func TestSummaryMarksUnsupportedIntelligenceUnavailable(t *testing.T) {
	t.Parallel()
	run := rie.NewRunContext(t.TempDir(), rie.DefaultConfig())
	if err := fullPipeline(t, true).Run(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	summary, _ := InventoryFrom(run)
	wantIDs := []string{"controllers", "coverage", "diagnostics", "services", "tests"}
	capabilities := summary.Capabilities()
	if len(capabilities) != len(wantIDs) {
		t.Fatalf("Capabilities = %#v", capabilities)
	}
	for index, capability := range capabilities {
		if capability.ID != wantIDs[index] || capability.Status != StatusUnavailable || capability.Reason == "" || capability.FutureOwner == "" {
			t.Errorf("Capability[%d] = %#v", index, capability)
		}
	}
}

func TestSummaryIsImmutableToConsumers(t *testing.T) {
	t.Parallel()
	repository := t.TempDir()
	mustWrite(t, filepath.Join(repository, "main.go"), "package main")
	run := rie.NewRunContext(repository, rie.DefaultConfig())
	if err := fullPipeline(t, true).Run(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	summary, _ := InventoryFrom(run)
	sections := summary.Sections()
	sections[0].ID = "changed"
	capabilities := summary.Capabilities()
	capabilities[0].ID = "changed"
	metadataCopy := summary.RepositoryMetadata()
	languages := metadataCopy.Languages()
	languages[0].Name = "changed"
	if summary.Sections()[0].ID == "changed" || summary.Capabilities()[0].ID == "changed" || summary.RepositoryMetadata().Languages()[0].Name == "changed" {
		t.Error("consumer mutation changed summary")
	}
}

func TestSummaryProducesValidEmptyRepositoryEntryPoint(t *testing.T) {
	t.Parallel()
	run := rie.NewRunContext(t.TempDir(), rie.DefaultConfig())
	if err := fullPipeline(t, true).Run(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	summary, _ := InventoryFrom(run)
	metadata := summary.RepositoryMetadata()
	if len(metadata.Languages()) != 0 || len(metadata.Frameworks()) != 0 || len(metadata.Build().PackageManagers()) != 0 || len(summary.Sections()) == 0 {
		t.Errorf("Summary = %#v", summary)
	}
	if len(run.Report.Warnings) != 0 || len(run.Report.Errors) != 0 {
		t.Errorf("diagnostics = %#v %#v", run.Report.Warnings, run.Report.Errors)
	}
}

func TestSummaryRequiresMetadataArtifact(t *testing.T) {
	t.Parallel()
	run := rie.NewRunContext(t.TempDir(), rie.DefaultConfig())
	if err := New().Execute(context.Background(), run); err != ErrMetadataRequired {
		t.Errorf("Execute() error = %v", err)
	}
}

func TestSummaryRejectsInvalidCapabilityDefinitions(t *testing.T) {
	t.Parallel()
	repository := t.TempDir()
	run := rie.NewRunContext(repository, rie.DefaultConfig())
	if err := fullPipeline(t, false).Run(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	tests := []Config{
		{UnavailableCapabilities: []CapabilityDefinition{{ID: ""}}},
		{UnavailableCapabilities: []CapabilityDefinition{{ID: "same"}, {ID: "same"}}},
	}
	for _, config := range tests {
		if err := New(config).Execute(context.Background(), run); !errors.Is(err, ErrInvalidCapability) {
			t.Errorf("Execute() error = %v", err)
		}
	}
}

func TestSummaryDoesNotConsumeMutableReportState(t *testing.T) {
	t.Parallel()
	repository := t.TempDir()
	run := rie.NewRunContext(repository, rie.DefaultConfig())
	if err := fullPipeline(t, false).Run(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	run.Report.Metadata.Name = "incorrect"
	if err := New().Execute(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	summary, _ := InventoryFrom(run)
	if summary.RepositoryMetadata().Repository().Name == "incorrect" {
		t.Error("summary consumed mutable report state")
	}
}

func TestSummaryEngineMetadata(t *testing.T) {
	t.Parallel()
	engine := New()
	if engine.Name() != "repository-intelligence-summary" || engine.Version() != "0.7.0" || engine.Description() == "" {
		t.Errorf("metadata = %s %s %q", engine.Name(), engine.Version(), engine.Description())
	}
}

func fullPipeline(t testing.TB, includeSummary bool) *rie.Pipeline {
	t.Helper()
	pipeline := rie.New()
	engines := []rie.Engine{discovery.New(), ignoreengine.New(), languageengine.New(), frameworkengine.New(), buildengine.New(), metadataengine.New()}
	for _, engine := range engines {
		mustRegister(t, pipeline, engine)
	}
	if includeSummary {
		mustRegister(t, pipeline, New())
	}
	return pipeline
}

func mustRegister(t testing.TB, pipeline *rie.Pipeline, engine rie.Engine) {
	t.Helper()
	if err := pipeline.Register(engine); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t testing.TB, filePath, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
