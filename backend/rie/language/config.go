package language

// Config maps lowercase file extensions to stable language names.
type Config struct {
	Extensions map[string]string
}

// DefaultConfig returns the extension set supported by RIE v0.3.
func DefaultConfig() Config {
	return Config{Extensions: map[string]string{
		".go":   "Go",
		".ts":   "TypeScript",
		".tsx":  "TypeScript",
		".js":   "JavaScript",
		".jsx":  "JavaScript",
		".py":   "Python",
		".java": "Java",
		".cs":   "C#",
		".sql":  "SQL",
	}}
}
