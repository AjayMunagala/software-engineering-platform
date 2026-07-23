package semantic

import (
	"fmt"
	"runtime"
)

const (
	defaultMaxSourceFileSize     int64 = 10 << 20
	defaultMaxPackageFiles             = 2_000
	defaultMaxPackageBytes       int64 = 256 << 20
	defaultMaxDiagnostics              = 1_000
	defaultMaxDiagnosticsPerFile       = 50
	defaultMaxRelationships            = 1_000_000
)

// Config bounds semantic work. Every zero field selects its documented default.
type Config struct {
	MaxWorkers            int   `json:"max_workers"`
	MaxSourceFileSize     int64 `json:"max_source_file_size"`
	MaxPackageFiles       int   `json:"max_package_files"`
	MaxPackageBytes       int64 `json:"max_package_bytes"`
	MaxDiagnostics        int   `json:"max_diagnostics"`
	MaxDiagnosticsPerFile int   `json:"max_diagnostics_per_file"`
	MaxRelationships      int   `json:"max_relationships"`
}

// DefaultConfig returns explicit bounded candidate defaults.
func DefaultConfig() Config {
	workers := runtime.GOMAXPROCS(0)
	if workers > 8 {
		workers = 8
	}
	if workers < 1 {
		workers = 1
	}
	return Config{
		MaxWorkers:            workers,
		MaxSourceFileSize:     defaultMaxSourceFileSize,
		MaxPackageFiles:       defaultMaxPackageFiles,
		MaxPackageBytes:       defaultMaxPackageBytes,
		MaxDiagnostics:        defaultMaxDiagnostics,
		MaxDiagnosticsPerFile: defaultMaxDiagnosticsPerFile,
		MaxRelationships:      defaultMaxRelationships,
	}
}

func (config Config) withDefaults() Config {
	defaults := DefaultConfig()
	if config.MaxWorkers == 0 {
		config.MaxWorkers = defaults.MaxWorkers
	}
	if config.MaxSourceFileSize == 0 {
		config.MaxSourceFileSize = defaults.MaxSourceFileSize
	}
	if config.MaxPackageFiles == 0 {
		config.MaxPackageFiles = defaults.MaxPackageFiles
	}
	if config.MaxPackageBytes == 0 {
		config.MaxPackageBytes = defaults.MaxPackageBytes
	}
	if config.MaxDiagnostics == 0 {
		config.MaxDiagnostics = defaults.MaxDiagnostics
	}
	if config.MaxDiagnosticsPerFile == 0 {
		config.MaxDiagnosticsPerFile = defaults.MaxDiagnosticsPerFile
	}
	if config.MaxRelationships == 0 {
		config.MaxRelationships = defaults.MaxRelationships
	}
	return config
}

// Validate rejects unsafe or contradictory candidate limits.
func (config Config) Validate() error {
	if config.MaxWorkers < 1 || config.MaxWorkers > 8 {
		return fmt.Errorf("%w: MaxWorkers must be between 1 and 8", ErrInvalidConfig)
	}
	if config.MaxSourceFileSize < 1 {
		return fmt.Errorf("%w: MaxSourceFileSize must be positive", ErrInvalidConfig)
	}
	if config.MaxPackageFiles < 1 {
		return fmt.Errorf("%w: MaxPackageFiles must be positive", ErrInvalidConfig)
	}
	if config.MaxPackageBytes < 1 {
		return fmt.Errorf("%w: MaxPackageBytes must be positive", ErrInvalidConfig)
	}
	if config.MaxDiagnostics < 1 {
		return fmt.Errorf("%w: MaxDiagnostics must be positive", ErrInvalidConfig)
	}
	if config.MaxDiagnosticsPerFile < 1 {
		return fmt.Errorf("%w: MaxDiagnosticsPerFile must be positive", ErrInvalidConfig)
	}
	if config.MaxRelationships < 1 {
		return fmt.Errorf("%w: MaxRelationships must be positive", ErrInvalidConfig)
	}
	return nil
}
