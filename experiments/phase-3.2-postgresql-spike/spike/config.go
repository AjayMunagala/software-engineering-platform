package spike

import (
	"fmt"
	"path/filepath"
	"strings"
)

const (
	defaultIterations = 10
	maximumIterations = 50
	maximumWorkers    = 8
)

// FixtureConfig controls one deterministic artifact generation pass.
type FixtureConfig struct {
	RepositoryRoot   string
	OutputDirectory  string
	Label            string
	Commit           string
	IncludeRIEReport bool
	SemanticOnly     bool
	MaxWorkers       int
}

func (config FixtureConfig) validate() (FixtureConfig, error) {
	root, err := filepath.Abs(strings.TrimSpace(config.RepositoryRoot))
	if err != nil || root == "" {
		return FixtureConfig{}, fmt.Errorf("%w: repository root", ErrInvalidConfig)
	}
	output, err := filepath.Abs(strings.TrimSpace(config.OutputDirectory))
	if err != nil || output == "" {
		return FixtureConfig{}, fmt.Errorf("%w: output directory", ErrInvalidConfig)
	}
	label := strings.TrimSpace(config.Label)
	if label == "" || !safeMachineName(label) {
		return FixtureConfig{}, fmt.Errorf("%w: fixture label", ErrInvalidConfig)
	}
	workers := config.MaxWorkers
	if workers == 0 {
		workers = maximumWorkers
	}
	if workers < 1 || workers > maximumWorkers {
		return FixtureConfig{}, fmt.Errorf("%w: workers must be 1-%d", ErrInvalidConfig, maximumWorkers)
	}
	config.RepositoryRoot = root
	config.OutputDirectory = output
	config.Label = label
	config.Commit = strings.TrimSpace(config.Commit)
	config.MaxWorkers = workers
	return config, nil
}

// Config controls the isolated database benchmark.
type Config struct {
	ConnectionString string
	FixtureDirectory string
	OutputPath       string
	Iterations       int
	HostStorage      string
}

func (config Config) validate() (Config, error) {
	config.ConnectionString = strings.TrimSpace(config.ConnectionString)
	if config.ConnectionString == "" {
		return Config{}, fmt.Errorf("%w: connection string", ErrInvalidConfig)
	}
	directory, err := filepath.Abs(strings.TrimSpace(config.FixtureDirectory))
	if err != nil || directory == "" {
		return Config{}, fmt.Errorf("%w: fixture directory", ErrInvalidConfig)
	}
	output, err := filepath.Abs(strings.TrimSpace(config.OutputPath))
	if err != nil || output == "" {
		return Config{}, fmt.Errorf("%w: output path", ErrInvalidConfig)
	}
	if config.Iterations == 0 {
		config.Iterations = defaultIterations
	}
	if config.Iterations < 1 || config.Iterations > maximumIterations {
		return Config{}, fmt.Errorf("%w: iterations must be 1-%d", ErrInvalidConfig, maximumIterations)
	}
	config.FixtureDirectory = directory
	config.OutputPath = output
	config.HostStorage = strings.TrimSpace(config.HostStorage)
	return config, nil
}

func safeMachineName(value string) bool {
	for _, character := range value {
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') || character == '-' || character == '_' {
			continue
		}
		return false
	}
	return value != ""
}
