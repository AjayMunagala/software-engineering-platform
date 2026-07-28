package adapters

import (
	"errors"
	"os"
	"time"
)

const (
	defaultBufferBytes      = 64 * 1024
	maximumPayloadBytes     = uint64(4) * 1024 * 1024 * 1024
	defaultCleanupTimeout   = 5 * time.Second
	defaultMaximumArtifacts = 64
)

var ErrInvalidConfig = errors.New("invalid intelligence adapter configuration")

type Config struct {
	SpoolDirectory   string
	BufferBytes      int
	MaxArtifactBytes uint64
	CleanupTimeout   time.Duration
	MaxArtifacts     int
}

func DefaultConfig() Config {
	return Config{SpoolDirectory: os.TempDir(), BufferBytes: defaultBufferBytes, MaxArtifactBytes: maximumPayloadBytes, CleanupTimeout: defaultCleanupTimeout, MaxArtifacts: defaultMaximumArtifacts}
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
	if config.CleanupTimeout == 0 {
		config.CleanupTimeout = defaults.CleanupTimeout
	}
	if config.MaxArtifacts == 0 {
		config.MaxArtifacts = defaults.MaxArtifacts
	}
	return config
}

func (config Config) Validate() error {
	if config.SpoolDirectory == "" || config.BufferBytes < 4*1024 || config.BufferBytes > 4*1024*1024 || config.MaxArtifactBytes == 0 || config.MaxArtifactBytes > maximumPayloadBytes || config.CleanupTimeout < time.Second || config.CleanupTimeout > time.Minute || config.MaxArtifacts < 1 || config.MaxArtifacts > defaultMaximumArtifacts {
		return ErrInvalidConfig
	}
	return nil
}
