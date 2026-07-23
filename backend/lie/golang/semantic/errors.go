package semantic

import "errors"

var (
	// ErrTooManyConfigs indicates that New received more than one Config.
	ErrTooManyConfigs = errors.New("too many semantic configurations")
	// ErrInvalidConfig indicates an unsafe or contradictory limit.
	ErrInvalidConfig = errors.New("invalid semantic configuration")
	// ErrContextRequired indicates a nil context.
	ErrContextRequired = errors.New("context is required")
	// ErrMissingRepositorySnapshot indicates an absent zero-value snapshot.
	ErrMissingRepositorySnapshot = errors.New("repository snapshot is required")
	// ErrIncompatibleRepositorySnapshot indicates an unsupported snapshot contract.
	ErrIncompatibleRepositorySnapshot = errors.New("repository snapshot is incompatible")
	// ErrMissingGoLanguageInventory indicates an absent zero-value syntax artifact.
	ErrMissingGoLanguageInventory = errors.New("Go language inventory is required")
	// ErrIncompatibleGoInventory indicates an unsupported syntax contract.
	ErrIncompatibleGoInventory = errors.New("Go language inventory is incompatible")
	// ErrMissingPackageIdentityInventory indicates an absent identity artifact.
	ErrMissingPackageIdentityInventory = errors.New("Go package identity inventory is required")
	// ErrIncompatiblePackageIdentity indicates an unsupported identity contract.
	ErrIncompatiblePackageIdentity = errors.New("Go package identity inventory is incompatible")
	// ErrArtifactProvenanceMismatch indicates structurally inconsistent prerequisites.
	ErrArtifactProvenanceMismatch = errors.New("semantic artifact provenance mismatch")
	// ErrInvalidRepositoryRoot indicates that the authorized root cannot be opened safely.
	ErrInvalidRepositoryRoot = errors.New("invalid semantic repository root")
	// ErrSourceOutsideRoot indicates a lexical or symbolic-link boundary violation.
	ErrSourceOutsideRoot = errors.New("semantic source is outside repository root")
	// ErrSourceMissing indicates an authorized path that no longer exists.
	ErrSourceMissing = errors.New("semantic source is missing")
	// ErrSourceUnreadable indicates source that cannot be read as a regular file.
	ErrSourceUnreadable = errors.New("semantic source is unreadable")
	// ErrSourceOversized indicates source beyond the configured byte limit.
	ErrSourceOversized = errors.New("semantic source exceeds configured size")
)
