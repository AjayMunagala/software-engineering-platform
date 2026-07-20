package golang

import (
	"fmt"
	"runtime"

	"github.com/AjayMunagala/software-engineering-platform/backend/lie"
)

// Config defines options for the Go Language Engine.
type Config struct {
	MaxWorkers        int   `json:"max_workers"`
	MaxSourceFileSize int64 `json:"max_source_file_size"`
	MaxDiagnostics    int   `json:"max_diagnostics"`
	IncludeTests      bool  `json:"include_tests"`
}

// DefaultConfig returns safe defaults for parsing Go repositories.
func DefaultConfig() Config {
	workers := runtime.NumCPU()
	if workers > 8 {
		workers = 8
	}
	if workers < 1 {
		workers = 1
	}
	return Config{
		MaxWorkers:        workers,
		MaxSourceFileSize: 10 * 1024 * 1024, // 10 MiB
		MaxDiagnostics:    1000,
		IncludeTests:      true,
	}
}

func (c Config) Validate() error {
	if c.MaxWorkers < 1 {
		return fmt.Errorf("%w: MaxWorkers must be positive", lie.ErrInvalidConfig)
	}
	if c.MaxWorkers > 8 {
		return fmt.Errorf("%w: MaxWorkers must not exceed 8", lie.ErrInvalidConfig)
	}
	if c.MaxSourceFileSize <= 0 {
		return fmt.Errorf("%w: MaxSourceFileSize must be positive", lie.ErrInvalidConfig)
	}
	if c.MaxDiagnostics < 1 {
		return fmt.Errorf("%w: MaxDiagnostics must be positive", lie.ErrInvalidConfig)
	}
	return nil
}
