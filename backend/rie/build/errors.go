package build

import "errors"

var (
	ErrSnapshotRequired    = errors.New("Build & Package Intelligence Engine requires RepositorySnapshot")
	ErrInvalidManifestSize = errors.New("Build & Package Intelligence Engine maximum manifest size must be positive")
	ErrNoDetectors         = errors.New("Build & Package Intelligence Engine requires at least one detector")
	ErrInvalidDetector     = errors.New("Build & Package Intelligence Engine detector ID must be unique and non-empty")
	ErrManifestTooLarge    = errors.New("build manifest exceeds configured size limit")
)
