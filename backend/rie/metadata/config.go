package metadata

// Config controls deterministic monorepo classification.
type Config struct {
	MonorepoMinimumProjects int
	MonorepoMinimumMembers  int
}

func DefaultConfig() Config {
	return Config{MonorepoMinimumProjects: 2, MonorepoMinimumMembers: 2}
}
