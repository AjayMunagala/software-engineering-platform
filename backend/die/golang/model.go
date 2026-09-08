package golang

import (
	"context"
	"github.com/AjayMunagala/software-engineering-platform/backend/die"
	syntax "github.com/AjayMunagala/software-engineering-platform/backend/lie/golang"
	identity "github.com/AjayMunagala/software-engineering-platform/backend/lie/golang/packageidentity"
	"github.com/AjayMunagala/software-engineering-platform/backend/lie/golang/semantic"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
)

type InputParams struct {
	RepositorySnapshot rie.RepositorySnapshot
	LanguageInventory  syntax.GoLanguageInventory
	PackageIdentity    identity.GoPackageIdentityInventory
	SemanticInventory  semantic.GoSemanticInventory
}

// Inputs holds only immutable artifact values; it exposes no host paths or views.
type Inputs struct {
	p     InputParams
	valid bool
}

func (Inputs) String() string   { return "Go dependency inputs (redacted)" }
func (Inputs) GoString() string { return "Go dependency inputs (redacted)" }
func NewInputs(ctx context.Context, p InputParams) (Inputs, error) {
	if err := check(ctx); err != nil {
		return Inputs{}, err
	}
	sm, lm, im, sem := p.RepositorySnapshot.Metadata(), p.LanguageInventory.Metadata(), p.PackageIdentity.Metadata(), p.SemanticInventory.Metadata()
	if sm.Name != rie.RepositorySnapshotArtifactName || sm.Version != "1.0.0" || lm.Name != syntax.ArtifactName || lm.Version != "1.0.0" || im.Name != identity.ArtifactName || im.Version != "1.0.0" || sem.Name != semantic.ArtifactName || sem.Version != "1.0.0" || im.IDSchemeVersion != identity.ProofIDSchemeVersion || sem.IDSchemeVersion != semantic.IDSchemeVersion {
		return Inputs{}, fail(die.ErrorInvalidInput, "incompatible_artifacts")
	}
	for _, v := range []struct {
		got  []rie.ArtifactReference
		want []string
	}{
		{p.LanguageInventory.SourceArtifacts(), []string{rie.RepositorySnapshotArtifactName}},
		{p.PackageIdentity.SourceArtifacts(), []string{rie.RepositorySnapshotArtifactName, syntax.ArtifactName}},
		{p.SemanticInventory.SourceArtifacts(), []string{rie.RepositorySnapshotArtifactName, syntax.ArtifactName, identity.ArtifactName}},
	} {
		if err := check(ctx); err != nil {
			return Inputs{}, err
		}
		seen := map[string]bool{}
		for _, r := range v.got {
			if seen[r.Name] || r.Version != "1.0.0" || !safe(r.Name) {
				return Inputs{}, fail(die.ErrorIntegrity, "invalid_prerequisites")
			}
			seen[r.Name] = true
		}
		for _, name := range v.want {
			if !seen[name] {
				return Inputs{}, fail(die.ErrorIntegrity, "missing_prerequisite")
			}
		}
	}
	return Inputs{p, true}, check(ctx)
}
