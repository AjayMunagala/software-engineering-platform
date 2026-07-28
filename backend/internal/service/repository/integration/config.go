package integration

import (
	"errors"
	"time"
)

var ErrInvalidConfig = errors.New("invalid repository service integration configuration")

type Config struct {
	FinalizationTimeout time.Duration
	ReadPageSize        int
	MaxArtifacts        int
	MaxDependencies     int
	MaxPayloadBytes     uint64
}

func DefaultConfig() Config {
	return Config{FinalizationTimeout: 5 * time.Second, ReadPageSize: 100, MaxArtifacts: 64, MaxDependencies: 4096, MaxPayloadBytes: uint64(4) << 30}
}

func (config Config) withDefaults() Config {
	defaults := DefaultConfig()
	if config.FinalizationTimeout == 0 {
		config.FinalizationTimeout = defaults.FinalizationTimeout
	}
	if config.ReadPageSize == 0 {
		config.ReadPageSize = defaults.ReadPageSize
	}
	if config.MaxArtifacts == 0 {
		config.MaxArtifacts = defaults.MaxArtifacts
	}
	if config.MaxDependencies == 0 {
		config.MaxDependencies = defaults.MaxDependencies
	}
	if config.MaxPayloadBytes == 0 {
		config.MaxPayloadBytes = defaults.MaxPayloadBytes
	}
	return config
}

func (config Config) Validate() error {
	if config.FinalizationTimeout < time.Second || config.FinalizationTimeout > time.Minute || config.ReadPageSize < 1 || config.ReadPageSize > 1000 || config.MaxArtifacts < 1 || config.MaxArtifacts > 64 || config.MaxDependencies < 1 || config.MaxDependencies > 4096 || config.MaxPayloadBytes == 0 || config.MaxPayloadBytes > uint64(4)<<30 {
		return ErrInvalidConfig
	}
	return nil
}
