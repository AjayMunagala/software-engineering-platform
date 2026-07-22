package spike

import "fmt"

const (
	defaultNodeCheckInterval         = 1024
	defaultRelationshipCheckInterval = 256
	defaultMaxDiagnosticsPerFile     = 8
	defaultMaxDiagnostics            = 64
)

// Config contains only limits needed to validate the proposed contracts.
// Zero means the documented default.
type Config struct {
	NodeCheckInterval         int
	RelationshipCheckInterval int
	MaxDiagnosticsPerFile     int
	MaxDiagnostics            int
}

// DefaultConfig returns explicit spike defaults.
func DefaultConfig() Config {
	return Config{
		NodeCheckInterval:         defaultNodeCheckInterval,
		RelationshipCheckInterval: defaultRelationshipCheckInterval,
		MaxDiagnosticsPerFile:     defaultMaxDiagnosticsPerFile,
		MaxDiagnostics:            defaultMaxDiagnostics,
	}
}

func (config Config) withDefaults() Config {
	defaults := DefaultConfig()
	if config.NodeCheckInterval == 0 {
		config.NodeCheckInterval = defaults.NodeCheckInterval
	}
	if config.RelationshipCheckInterval == 0 {
		config.RelationshipCheckInterval = defaults.RelationshipCheckInterval
	}
	if config.MaxDiagnosticsPerFile == 0 {
		config.MaxDiagnosticsPerFile = defaults.MaxDiagnosticsPerFile
	}
	if config.MaxDiagnostics == 0 {
		config.MaxDiagnostics = defaults.MaxDiagnostics
	}
	return config
}

// Validate rejects negative or internally inconsistent limits.
func (config Config) Validate() error {
	if config.NodeCheckInterval <= 0 {
		return fmt.Errorf("%w: node checkpoint interval must be positive", ErrInvalidConfig)
	}
	if config.RelationshipCheckInterval <= 0 {
		return fmt.Errorf("%w: relationship checkpoint interval must be positive", ErrInvalidConfig)
	}
	if config.MaxDiagnosticsPerFile <= 0 {
		return fmt.Errorf("%w: per-file diagnostic limit must be positive", ErrInvalidConfig)
	}
	if config.MaxDiagnostics <= 0 {
		return fmt.Errorf("%w: global diagnostic limit must be positive", ErrInvalidConfig)
	}
	return nil
}
