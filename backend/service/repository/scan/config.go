package scan

import (
	"fmt"
	"time"
)

const (
	defaultCleanupTimeout      = 5 * time.Second
	defaultFinalizationTimeout = 5 * time.Second
	defaultMaxArtifacts        = 64
)

type Config struct {
	CleanupTimeout      time.Duration
	FinalizationTimeout time.Duration
	MaxArtifacts        int
}

func DefaultConfig() Config {
	return Config{CleanupTimeout: defaultCleanupTimeout, FinalizationTimeout: defaultFinalizationTimeout, MaxArtifacts: defaultMaxArtifacts}
}

func (config Config) withDefaults() Config {
	if config.CleanupTimeout == 0 {
		config.CleanupTimeout = defaultCleanupTimeout
	}
	if config.FinalizationTimeout == 0 {
		config.FinalizationTimeout = defaultFinalizationTimeout
	}
	if config.MaxArtifacts == 0 {
		config.MaxArtifacts = defaultMaxArtifacts
	}
	return config
}

func (config Config) Validate() error {
	if config.CleanupTimeout < time.Second || config.CleanupTimeout > time.Minute {
		return fmt.Errorf("%w: CleanupTimeout must be between 1 second and 1 minute", ErrInvalidConfig)
	}
	if config.FinalizationTimeout < time.Second || config.FinalizationTimeout > time.Minute {
		return fmt.Errorf("%w: FinalizationTimeout must be between 1 second and 1 minute", ErrInvalidConfig)
	}
	if config.MaxArtifacts < 1 || config.MaxArtifacts > defaultMaxArtifacts {
		return fmt.Errorf("%w: MaxArtifacts must be between 1 and %d", ErrInvalidConfig, defaultMaxArtifacts)
	}
	return nil
}
