package repository

import (
	"fmt"
	"time"
)

const (
	defaultFinalizationTimeout            = 5 * time.Second
	defaultMaxArtifactsPerScan            = 64
	defaultMaxArtifactBytes        uint64 = 4 << 30
	defaultMaterializerMemoryBytes        = 64 << 20
	defaultMaxDiagnostics                 = 10_000
	defaultMaxConcurrentScans             = 64
	defaultMaxPageSize                    = 1_000
	defaultMaxDisplayNameBytes            = 256
	defaultMaxSourceHandleBytes           = 1_024
)

// Config contains service-neutral validation/resource limits. Database URLs,
// credentials, pools, engines, and transport settings do not belong here.
type Config struct {
	FinalizationTimeout     time.Duration
	MaxArtifactsPerScan     int
	MaxArtifactBytes        uint64
	MaterializerMemoryBytes uint64
	MaxDiagnostics          int
	MaxConcurrentScans      int
	MaxPageSize             int
	MaxDisplayNameBytes     int
	MaxSourceHandleBytes    int
}

func DefaultConfig() Config {
	return Config{
		FinalizationTimeout: defaultFinalizationTimeout, MaxArtifactsPerScan: defaultMaxArtifactsPerScan,
		MaxArtifactBytes: defaultMaxArtifactBytes, MaterializerMemoryBytes: defaultMaterializerMemoryBytes,
		MaxDiagnostics: defaultMaxDiagnostics, MaxConcurrentScans: defaultMaxConcurrentScans,
		MaxPageSize: defaultMaxPageSize, MaxDisplayNameBytes: defaultMaxDisplayNameBytes,
		MaxSourceHandleBytes: defaultMaxSourceHandleBytes,
	}
}

func (config Config) withDefaults() Config {
	defaults := DefaultConfig()
	if config.FinalizationTimeout == 0 {
		config.FinalizationTimeout = defaults.FinalizationTimeout
	}
	if config.MaxArtifactsPerScan == 0 {
		config.MaxArtifactsPerScan = defaults.MaxArtifactsPerScan
	}
	if config.MaxArtifactBytes == 0 {
		config.MaxArtifactBytes = defaults.MaxArtifactBytes
	}
	if config.MaterializerMemoryBytes == 0 {
		config.MaterializerMemoryBytes = defaults.MaterializerMemoryBytes
	}
	if config.MaxDiagnostics == 0 {
		config.MaxDiagnostics = defaults.MaxDiagnostics
	}
	if config.MaxConcurrentScans == 0 {
		config.MaxConcurrentScans = defaults.MaxConcurrentScans
	}
	if config.MaxPageSize == 0 {
		config.MaxPageSize = defaults.MaxPageSize
	}
	if config.MaxDisplayNameBytes == 0 {
		config.MaxDisplayNameBytes = defaults.MaxDisplayNameBytes
	}
	if config.MaxSourceHandleBytes == 0 {
		config.MaxSourceHandleBytes = defaults.MaxSourceHandleBytes
	}
	return config
}

func (config Config) Validate() error {
	checks := []struct {
		valid   bool
		message string
	}{
		{config.FinalizationTimeout >= time.Second && config.FinalizationTimeout <= time.Minute, "FinalizationTimeout must be between 1 second and 1 minute"},
		{config.MaxArtifactsPerScan >= 1 && config.MaxArtifactsPerScan <= defaultMaxArtifactsPerScan, "MaxArtifactsPerScan must be between 1 and 64"},
		{config.MaxArtifactBytes >= 1 && config.MaxArtifactBytes <= defaultMaxArtifactBytes, "MaxArtifactBytes must be between 1 byte and 4 GiB"},
		{config.MaterializerMemoryBytes >= 1<<20 && config.MaterializerMemoryBytes <= defaultMaterializerMemoryBytes, "MaterializerMemoryBytes must be between 1 MiB and 64 MiB"},
		{config.MaxDiagnostics >= 1 && config.MaxDiagnostics <= defaultMaxDiagnostics, "MaxDiagnostics must be between 1 and 10000"},
		{config.MaxConcurrentScans >= 1 && config.MaxConcurrentScans <= defaultMaxConcurrentScans, "MaxConcurrentScans must be between 1 and 64"},
		{config.MaxPageSize >= 1 && config.MaxPageSize <= defaultMaxPageSize, "MaxPageSize must be between 1 and 1000"},
		{config.MaxDisplayNameBytes >= 1 && config.MaxDisplayNameBytes <= defaultMaxDisplayNameBytes, "MaxDisplayNameBytes must be between 1 and 256"},
		{config.MaxSourceHandleBytes >= 1 && config.MaxSourceHandleBytes <= defaultMaxSourceHandleBytes, "MaxSourceHandleBytes must be between 1 and 1024"},
	}
	for _, check := range checks {
		if !check.valid {
			return fmt.Errorf("%w: %s", ErrInvalidConfig, check.message)
		}
	}
	return nil
}
