package packageidentity

import "errors"

var (
	ErrInvalidConfig                  = errors.New("invalid package identity configuration")
	ErrContextRequired                = errors.New("context is required")
	ErrRepositorySnapshotRequired     = errors.New("repository snapshot is required")
	ErrRepositorySnapshotIncompatible = errors.New("repository snapshot is incompatible")
	ErrGoInventoryRequired            = errors.New("Go language inventory is required")
	ErrGoInventoryIncompatible        = errors.New("Go language inventory is incompatible")
	ErrArtifactProvenanceMismatch     = errors.New("artifact provenance mismatch")
	ErrRepositoryRootInvalid          = errors.New("repository root is invalid")
	ErrManifestOutsideRoot            = errors.New("manifest path is outside repository root")
	ErrManifestMissing                = errors.New("manifest is missing")
	ErrManifestUnreadable             = errors.New("manifest is unreadable")
	ErrManifestOversized              = errors.New("manifest exceeds configured size")
)
