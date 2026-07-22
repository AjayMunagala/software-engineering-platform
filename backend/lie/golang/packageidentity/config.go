package packageidentity

import (
	"fmt"
	"runtime"
)

const (
	defaultMaxManifestSize       int64 = 4 << 20
	defaultMaxDiagnostics              = 1_000
	defaultMaxDiagnosticsPerFile       = 50
)

// Config bounds manifest parsing and diagnostics. Zero means the default.
type Config struct {
	MaxWorkers            int
	MaxManifestSize       int64
	MaxDiagnostics        int
	MaxDiagnosticsPerFile int
}

// DefaultConfig returns explicit safe defaults.
func DefaultConfig() Config {
	return Config{
		MaxWorkers:            min(runtime.GOMAXPROCS(0), 8),
		MaxManifestSize:       defaultMaxManifestSize,
		MaxDiagnostics:        defaultMaxDiagnostics,
		MaxDiagnosticsPerFile: defaultMaxDiagnosticsPerFile,
	}
}

func (config Config) withDefaults() Config {
	defaults := DefaultConfig()
	if config.MaxWorkers == 0 {
		config.MaxWorkers = defaults.MaxWorkers
	}
	if config.MaxManifestSize == 0 {
		config.MaxManifestSize = defaults.MaxManifestSize
	}
	if config.MaxDiagnostics == 0 {
		config.MaxDiagnostics = defaults.MaxDiagnostics
	}
	if config.MaxDiagnosticsPerFile == 0 {
		config.MaxDiagnosticsPerFile = defaults.MaxDiagnosticsPerFile
	}
	return config
}

// Validate rejects unsafe or contradictory limits.
func (config Config) Validate() error {
	if config.MaxWorkers < 1 || config.MaxWorkers > 8 {
		return fmt.Errorf("%w: MaxWorkers must be between 1 and 8", ErrInvalidConfig)
	}
	if config.MaxManifestSize < 1 {
		return fmt.Errorf("%w: MaxManifestSize must be positive", ErrInvalidConfig)
	}
	if config.MaxDiagnostics < 1 {
		return fmt.Errorf("%w: MaxDiagnostics must be positive", ErrInvalidConfig)
	}
	if config.MaxDiagnosticsPerFile < 1 {
		return fmt.Errorf("%w: MaxDiagnosticsPerFile must be positive", ErrInvalidConfig)
	}
	return nil
}
