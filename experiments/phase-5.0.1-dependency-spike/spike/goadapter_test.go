package spike

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/AjayMunagala/software-engineering-platform/backend/lie"
	golang "github.com/AjayMunagala/software-engineering-platform/backend/lie/golang"
	"github.com/AjayMunagala/software-engineering-platform/backend/lie/golang/packageidentity"
	"github.com/AjayMunagala/software-engineering-platform/backend/lie/golang/semantic"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie/discovery"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie/ignore"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie/language"
)

func TestNormalizeReleasedGoArtifacts(t *testing.T) {
	snapshot, syntax, identities, semantics := releasedArtifacts(t, map[string]string{
		"go.work":     "go 1.26\n\nuse (\n\t./app\n\t./lib\n)\n",
		"app/go.mod":  "module example.com/app\n\ngo 1.26\n\nrequire example.com/lib v0.0.0\n\nreplace example.com/lib => ../lib\n",
		"app/main.go": "package main\nimport \"example.com/lib\"\nfunc main() { lib.Value() }\n",
		"lib/go.mod":  "module example.com/lib\n\ngo 1.26\n",
		"lib/lib.go":  "package lib\nfunc Value() int { return 1 }\n",
	})
	beforeSyntax, _ := json.Marshal(syntax)
	beforeIdentities, _ := json.Marshal(identities)
	beforeSemantics, _ := json.Marshal(semantics)
	input, err := NormalizeReleasedGo(snapshot, syntax, identities, semantics)
	if err != nil {
		t.Fatal(err)
	}
	result, err := mustRunner(t, Config{Workers: 8}).Run(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}

	graphs := map[GraphKind]int{}
	for _, edge := range result.Edges {
		graphs[edge.Graph]++
	}
	if graphs[GraphPackage] == 0 {
		t.Fatalf("released import binding did not produce package edge: %#v", result.Edges)
	}
	if graphs[GraphModule] == 0 {
		t.Fatalf("workspace modules did not produce module edge: %#v", result.Edges)
	}
	afterSyntax, _ := json.Marshal(syntax)
	afterIdentities, _ := json.Marshal(identities)
	afterSemantics, _ := json.Marshal(semantics)
	if !bytes.Equal(beforeSyntax, afterSyntax) || !bytes.Equal(beforeIdentities, afterIdentities) || !bytes.Equal(beforeSemantics, afterSemantics) {
		t.Fatal("adapter mutated a released artifact")
	}
}

func releasedArtifacts(t testing.TB, files map[string]string) (rie.RepositorySnapshot, golang.GoLanguageInventory, packageidentity.GoPackageIdentityInventory, semantic.GoSemanticInventory) {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		absolute := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(absolute, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	run := rie.NewRunContext(root, rie.DefaultConfig())
	pipeline := rie.New()
	for _, engine := range []rie.Engine{discovery.New(), ignore.New(), language.New()} {
		if err := pipeline.Register(engine); err != nil {
			t.Fatal(err)
		}
	}
	if err := pipeline.Run(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	snapshot, ok := rie.ArtifactAs[rie.RepositorySnapshot](run.Artifacts, rie.RepositorySnapshotArtifactName)
	if !ok {
		t.Fatal("snapshot unavailable")
	}
	languages, ok := rie.ArtifactAs[language.LanguageInventory](run.Artifacts, language.LanguageInventoryArtifactName)
	if !ok {
		t.Fatal("languages unavailable")
	}
	goEngine, err := golang.New()
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := goEngine.Analyze(context.Background(), lie.Input{Snapshot: snapshot, Languages: languages})
	if err != nil {
		t.Fatal(err)
	}
	syntax, ok := artifact.(golang.GoLanguageInventory)
	if !ok {
		t.Fatalf("syntax type %T", artifact)
	}
	identityEngine, err := packageidentity.New()
	if err != nil {
		t.Fatal(err)
	}
	identities, err := identityEngine.Analyze(context.Background(), packageidentity.Input{Snapshot: snapshot, Syntax: syntax})
	if err != nil {
		t.Fatal(err)
	}
	semanticEngine, err := semantic.New()
	if err != nil {
		t.Fatal(err)
	}
	semantics, err := semanticEngine.Resolve(context.Background(), semantic.Input{Snapshot: snapshot, Syntax: syntax, PackageIdentities: identities})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot, syntax, identities, semantics
}
