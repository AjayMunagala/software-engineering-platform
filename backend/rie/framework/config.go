package framework

// Config controls bounded manifest inspection.
type Config struct {
	ManifestNames   []string
	MaxManifestSize int64
}

// DefaultConfig returns the deterministic manifest set supported by RIE v0.4.
func DefaultConfig() Config {
	return Config{
		ManifestNames: []string{
			"go.mod", "package.json", "pom.xml", "cargo.toml",
			"composer.json", "requirements.txt",
		},
		MaxManifestSize: 5 * 1024 * 1024,
	}
}
