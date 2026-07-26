package repositoryservice

import (
	"fmt"
	"os"
)

const (
	defaultBufferBytes  = 64 * 1024
	maximumPayloadBytes = uint64(4) * 1024 * 1024 * 1024
)

// Config bounds experimental materialization resources.
type Config struct {
	SpoolDirectory   string
	BufferBytes      int
	MaxArtifactBytes uint64
}

// DefaultConfig keeps spools outside repositories and within the accepted
// persistence limit.
func DefaultConfig() Config {
	return Config{
		SpoolDirectory:   os.TempDir(),
		BufferBytes:      defaultBufferBytes,
		MaxArtifactBytes: maximumPayloadBytes,
	}
}

func (config Config) withDefaults() Config {
	defaults := DefaultConfig()
	if config.SpoolDirectory == "" {
		config.SpoolDirectory = defaults.SpoolDirectory
	}
	if config.BufferBytes == 0 {
		config.BufferBytes = defaults.BufferBytes
	}
	if config.MaxArtifactBytes == 0 {
		config.MaxArtifactBytes = defaults.MaxArtifactBytes
	}
	return config
}

// Validate rejects unbounded or persistence-incompatible settings.
func (config Config) Validate() error {
	if config.SpoolDirectory == "" {
		return fmt.Errorf("%w: spool directory is required", ErrInvalidConfig)
	}
	if config.BufferBytes < 4*1024 || config.BufferBytes > 4*1024*1024 {
		return fmt.Errorf("%w: buffer bytes must be between 4 KiB and 4 MiB", ErrInvalidConfig)
	}
	if config.MaxArtifactBytes == 0 || config.MaxArtifactBytes > maximumPayloadBytes {
		return fmt.Errorf("%w: artifact limit must be within 1 byte and 4 GiB", ErrInvalidConfig)
	}
	return nil
}
