package discovery

// Config controls behavior owned only by Discovery Engine.
type Config struct {
	ExcludeGitMetadata bool
}

// DefaultConfig excludes internal Git metadata from repository entries while
// still detecting whether the repository uses Git.
func DefaultConfig() Config {
	return Config{ExcludeGitMetadata: true}
}
