package lie

import "errors"

var (
	ErrContextRequired           = errors.New("context is required")
	ErrArtifactStoreRequired     = errors.New("artifact store is required")
	ErrSnapshotRequired          = errors.New("RepositorySnapshot 1.0.0 artifact is required")
	ErrLanguageInventoryRequired = errors.New("LanguageInventory 1.0.0 artifact is required")
	ErrArtifactVersionMismatch   = errors.New("prerequisite artifact version mismatch")
	ErrNilEngine                 = errors.New("nil engine registered")
	ErrDuplicateEngine           = errors.New("duplicate language engine registered")
	ErrDuplicateArtifactName     = errors.New("duplicate artifact name registered")
	ErrInvalidConfig             = errors.New("invalid configuration")
	ErrLanguageInventoryMismatch = errors.New("language inventory mismatch")
	ErrInvalidEngineMetadata     = errors.New("invalid language engine metadata")
	ErrArtifactRequired          = errors.New("language engine returned no artifact")
	ErrArtifactContractMismatch  = errors.New("language artifact does not match engine contract")
)
