package rie

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type testEngine struct {
	name  string
	calls *[]string
	err   error
}

func (engine testEngine) Name() string        { return engine.name }
func (engine testEngine) Version() string     { return "test" }
func (engine testEngine) Description() string { return "pipeline test engine" }
func (engine testEngine) Execute(_ context.Context, _ *RunContext) error {
	*engine.calls = append(*engine.calls, engine.name)
	return engine.err
}

func TestPipelineExecutesConfiguredOrder(t *testing.T) {
	t.Parallel()

	var calls []string
	pipeline := New()
	for _, engine := range []Engine{
		testEngine{name: "discovery", calls: &calls},
		testEngine{name: "ignore", calls: &calls},
		testEngine{name: "language", calls: &calls},
	} {
		if err := pipeline.Register(engine); err != nil {
			t.Fatalf("Register(%s): %v", engine.Name(), err)
		}
	}
	run := NewRunContext(t.TempDir(), DefaultConfig())

	if err := pipeline.Run(context.Background(), run); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if want := []string{"discovery", "ignore", "language"}; !reflect.DeepEqual(calls, want) {
		t.Errorf("calls = %v, want %v", calls, want)
	}
	if run.Report.SchemaVersion != SchemaVersion {
		t.Errorf("SchemaVersion = %q, want %q", run.Report.SchemaVersion, SchemaVersion)
	}
	if run.Report.Scan.ID == "" || run.Report.Scan.FinishedAt.IsZero() {
		t.Error("scan metadata was not completed")
	}
	if len(run.Report.Scan.Engines) != 3 || run.Report.Scan.Engines[1].Name != "ignore" {
		t.Errorf("Scan.Engines = %#v", run.Report.Scan.Engines)
	}
	if run.Report.Warnings == nil || run.Report.Errors == nil {
		t.Error("diagnostic collections must be initialized")
	}
}

func TestPipelineStopsAndRecordsEngineError(t *testing.T) {
	t.Parallel()

	var calls []string
	pipeline := New()
	for _, engine := range []Engine{
		testEngine{name: "first", calls: &calls, err: errors.New("broken")},
		testEngine{name: "second", calls: &calls},
	} {
		if err := pipeline.Register(engine); err != nil {
			t.Fatalf("Register(%s): %v", engine.Name(), err)
		}
	}
	run := NewRunContext(t.TempDir(), DefaultConfig())

	if err := pipeline.Run(context.Background(), run); err == nil {
		t.Fatal("Run() error = nil, want error")
	}
	if want := []string{"first"}; !reflect.DeepEqual(calls, want) {
		t.Errorf("calls = %v, want %v", calls, want)
	}
	if len(run.Report.Errors) != 1 || run.Report.Errors[0].Engine != "first" {
		t.Errorf("Errors = %#v", run.Report.Errors)
	}
}

func TestPipelineRejectsDuplicateEngine(t *testing.T) {
	t.Parallel()

	var calls []string
	pipeline := New()
	if err := pipeline.Register(testEngine{name: "discovery", calls: &calls}); err != nil {
		t.Fatalf("first Register() error = %v", err)
	}
	if err := pipeline.Register(testEngine{name: "discovery", calls: &calls}); err == nil {
		t.Fatal("duplicate Register() error = nil, want error")
	}
}

func TestPipelineRejectsInvalidConfiguration(t *testing.T) {
	t.Parallel()

	run := NewRunContext(t.TempDir(), Config{})
	if err := New().Run(context.Background(), run); err == nil {
		t.Fatal("Run() error = nil, want invalid configuration error")
	}
	if len(run.Report.Errors) != 1 || run.Report.Errors[0].Code != "invalid_configuration" {
		t.Errorf("Errors = %#v", run.Report.Errors)
	}
}
