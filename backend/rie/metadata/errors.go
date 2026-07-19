package metadata

import "errors"

var (
	ErrDiscoveryRequired = errors.New("Repository Metadata Engine requires DiscoveryInventory")
	ErrSnapshotRequired  = errors.New("Repository Metadata Engine requires RepositorySnapshot")
	ErrLanguageRequired  = errors.New("Repository Metadata Engine requires LanguageInventory")
	ErrFrameworkRequired = errors.New("Repository Metadata Engine requires FrameworkInventory")
	ErrBuildRequired     = errors.New("Repository Metadata Engine requires BuildInventory")
	ErrInvalidConfig     = errors.New("Repository Metadata Engine monorepo thresholds must be positive")
)
