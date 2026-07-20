package lie

import (
	"context"

	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie/language"
)

// LanguageArtifact is the interface satisfied by every language-specific LIE artifact.
type LanguageArtifact interface {
	rie.Artifact
	Language() string
}

// Input carries immutable prerequisite artifacts for LIE engines.
type Input struct {
	Snapshot  rie.RepositorySnapshot
	Languages language.LanguageInventory
}

// Engine is the contract implemented by each language-specific syntax parser.
type Engine interface {
	Name() string
	Version() string
	Language() string
	ArtifactName() string
	Description() string
	Analyze(context.Context, Input) (LanguageArtifact, error)
}
