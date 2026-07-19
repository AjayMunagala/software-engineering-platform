package rie

import (
	"errors"
	"runtime"
)

// Config defines behavior shared by every RIE engine. Fields may be used only
// when the corresponding versioned engine supports them.
type Config struct {
	MaxWorkers     int      `json:"max_workers"`
	IgnorePatterns []string `json:"ignore_patterns"`
	FollowSymlinks bool     `json:"follow_symlinks"`
	MaxFileSize    int64    `json:"max_file_size"`
	ScanHidden     bool     `json:"scan_hidden"`
}

// DefaultConfig returns conservative local-scan defaults.
func DefaultConfig() Config {
	return Config{
		MaxWorkers:     runtime.NumCPU(),
		IgnorePatterns: []string{},
		FollowSymlinks: false,
		MaxFileSize:    10 * 1024 * 1024,
		ScanHidden:     true,
	}
}

// Validate rejects configuration that no component could safely interpret.
func (config Config) Validate() error {
	if config.MaxWorkers < 1 {
		return errors.New("max workers must be at least 1")
	}
	if config.MaxFileSize < 0 {
		return errors.New("max file size cannot be negative")
	}
	return nil
}

// NewRunContext creates an initialized pipeline context.
func NewRunContext(repositoryPath string, config Config) *RunContext {
	return &RunContext{
		RepositoryPath: repositoryPath,
		Config:         config,
	}
}
