package ignore

// Config controls behavior owned only by Ignore Engine.
type Config struct {
	IgnoreFileNames []string
}

// DefaultConfig loads Git-compatible ignore rules from .gitignore files.
func DefaultConfig() Config {
	return Config{IgnoreFileNames: []string{".gitignore"}}
}
