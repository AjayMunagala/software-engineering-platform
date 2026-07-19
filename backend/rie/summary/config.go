package summary

// CapabilityDefinition declares an intentionally unavailable intelligence fact.
type CapabilityDefinition struct {
	ID          string
	Label       string
	Reason      string
	FutureOwner string
}

type Config struct{ UnavailableCapabilities []CapabilityDefinition }

func DefaultConfig() Config {
	return Config{UnavailableCapabilities: []CapabilityDefinition{
		{ID: "controllers", Label: "Controllers", Reason: "requires code-structure classification", FutureOwner: "Language/Architecture Intelligence"},
		{ID: "services", Label: "Services", Reason: "requires architecture classification", FutureOwner: "Architecture Intelligence"},
		{ID: "tests", Label: "Tests", Reason: "requires test-file and test-symbol intelligence", FutureOwner: "Test Intelligence"},
		{ID: "coverage", Label: "Coverage", Reason: "requires approved coverage execution or imported results", FutureOwner: "Validation Intelligence"},
		{ID: "diagnostics", Label: "Complete diagnostics", Reason: "requires a cross-engine diagnostic artifact", FutureOwner: "RIE Stabilization"},
	}}
}
