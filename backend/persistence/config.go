package persistence

import "fmt"

const (
	defaultMaxPayloadBytes            ByteCount = 4 << 30
	maximumSchemaPayloadBytes         ByteCount = 8 << 30
	defaultMaxArtifactsPerPublication           = 256
	defaultMaxDependenciesPerArtifact           = 4_096
	defaultMaxProjectionBytes         ByteCount = 8 << 20
	defaultMaxAttributes                        = 128
	defaultMaxDiagnostics                       = 10_000
	defaultMaxStatistics                        = 10_000
	defaultMaxPageSize                          = 1_000
	defaultMaxRetentionBatch                    = 1_000
)

// Config contains storage-neutral validation limits. Database addresses,
// credentials, drivers, pools, and timeouts do not belong here.
type Config struct {
	MaxPayloadBytes            ByteCount
	MaxArtifactsPerPublication int
	MaxDependenciesPerArtifact int
	MaxProjectionBytes         ByteCount
	MaxAttributes              int
	MaxDiagnostics             int
	MaxStatistics              int
	MaxPageSize                int
	MaxRetentionBatch          int
}

// DefaultConfig returns the accepted Phase 3.4.1 limits.
func DefaultConfig() Config {
	return Config{
		MaxPayloadBytes:            defaultMaxPayloadBytes,
		MaxArtifactsPerPublication: defaultMaxArtifactsPerPublication,
		MaxDependenciesPerArtifact: defaultMaxDependenciesPerArtifact,
		MaxProjectionBytes:         defaultMaxProjectionBytes,
		MaxAttributes:              defaultMaxAttributes,
		MaxDiagnostics:             defaultMaxDiagnostics,
		MaxStatistics:              defaultMaxStatistics,
		MaxPageSize:                defaultMaxPageSize,
		MaxRetentionBatch:          defaultMaxRetentionBatch,
	}
}

func (config Config) withDefaults() Config {
	defaults := DefaultConfig()
	if config.MaxPayloadBytes == 0 {
		config.MaxPayloadBytes = defaults.MaxPayloadBytes
	}
	if config.MaxArtifactsPerPublication == 0 {
		config.MaxArtifactsPerPublication = defaults.MaxArtifactsPerPublication
	}
	if config.MaxDependenciesPerArtifact == 0 {
		config.MaxDependenciesPerArtifact = defaults.MaxDependenciesPerArtifact
	}
	if config.MaxProjectionBytes == 0 {
		config.MaxProjectionBytes = defaults.MaxProjectionBytes
	}
	if config.MaxAttributes == 0 {
		config.MaxAttributes = defaults.MaxAttributes
	}
	if config.MaxDiagnostics == 0 {
		config.MaxDiagnostics = defaults.MaxDiagnostics
	}
	if config.MaxStatistics == 0 {
		config.MaxStatistics = defaults.MaxStatistics
	}
	if config.MaxPageSize == 0 {
		config.MaxPageSize = defaults.MaxPageSize
	}
	if config.MaxRetentionBatch == 0 {
		config.MaxRetentionBatch = defaults.MaxRetentionBatch
	}
	return config
}

// Validate rejects limits that weaken the frozen physical contract.
func (config Config) Validate() error {
	if config.MaxPayloadBytes < 1 || config.MaxPayloadBytes > maximumSchemaPayloadBytes {
		return fmt.Errorf("%w: MaxPayloadBytes must be between 1 and %d", ErrInvalidConfig, maximumSchemaPayloadBytes)
	}
	if config.MaxArtifactsPerPublication < 1 || config.MaxArtifactsPerPublication > defaultMaxArtifactsPerPublication {
		return fmt.Errorf("%w: MaxArtifactsPerPublication must be between 1 and %d", ErrInvalidConfig, defaultMaxArtifactsPerPublication)
	}
	if config.MaxDependenciesPerArtifact < 1 || config.MaxDependenciesPerArtifact > defaultMaxDependenciesPerArtifact {
		return fmt.Errorf("%w: MaxDependenciesPerArtifact must be between 1 and %d", ErrInvalidConfig, defaultMaxDependenciesPerArtifact)
	}
	if config.MaxProjectionBytes < 1 || config.MaxProjectionBytes > defaultMaxProjectionBytes {
		return fmt.Errorf("%w: MaxProjectionBytes must be between 1 and %d", ErrInvalidConfig, defaultMaxProjectionBytes)
	}
	if config.MaxAttributes < 1 || config.MaxAttributes > defaultMaxAttributes {
		return fmt.Errorf("%w: MaxAttributes must be between 1 and %d", ErrInvalidConfig, defaultMaxAttributes)
	}
	if config.MaxDiagnostics < 1 || config.MaxDiagnostics > defaultMaxDiagnostics {
		return fmt.Errorf("%w: MaxDiagnostics must be between 1 and %d", ErrInvalidConfig, defaultMaxDiagnostics)
	}
	if config.MaxStatistics < 1 || config.MaxStatistics > defaultMaxStatistics {
		return fmt.Errorf("%w: MaxStatistics must be between 1 and %d", ErrInvalidConfig, defaultMaxStatistics)
	}
	if config.MaxPageSize < 1 || config.MaxPageSize > defaultMaxPageSize {
		return fmt.Errorf("%w: MaxPageSize must be between 1 and %d", ErrInvalidConfig, defaultMaxPageSize)
	}
	if config.MaxRetentionBatch < 1 || config.MaxRetentionBatch > defaultMaxRetentionBatch {
		return fmt.Errorf("%w: MaxRetentionBatch must be between 1 and %d", ErrInvalidConfig, defaultMaxRetentionBatch)
	}
	return nil
}
