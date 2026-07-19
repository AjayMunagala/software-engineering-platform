package build

// Config controls bounded manifest inspection and the in-code detector registry.
type Config struct {
	MaxManifestSize int64
	Detectors       []Detector
}

// DefaultConfig returns the deterministic detector registry supported by v0.5.
func DefaultConfig() Config {
	return Config{MaxManifestSize: 5 * 1024 * 1024, Detectors: defaultDetectors()}
}
