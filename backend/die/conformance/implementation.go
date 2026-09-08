package conformance

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/AjayMunagala/software-engineering-platform/backend/die"
)

func Run(t *testing.T, config Config) (Result, error) {
	t.Helper()
	if config.Factory == nil {
		return Result{}, ErrMissingFactory
	}
	cfg, err := die.NewConfig(die.ConfigParams{MaxNodes: 2})
	if err != nil {
		return Result{}, err
	}
	core, err := config.Factory(cfg)
	if err != nil {
		return Result{}, err
	}
	source := die.SourceIdentity{ArtifactName: "fixture", ArtifactVersion: "1.0.0", SourceID: "one"}
	node := die.NodeCandidate{Identity: die.NodeIdentity{Kind: die.NodePackage, Language: "Go", QualifiedName: "example.com/one", RepositoryPath: "one", Resolution: die.ResolvedLocal}, Name: "one", SourceIdentity: source}
	input := die.GraphInput{Nodes: []die.NodeCandidate{node}}
	first, err := core.Normalize(context.Background(), input)
	if err != nil {
		return Result{}, err
	}
	second, err := core.Normalize(context.Background(), input)
	if err != nil {
		return Result{}, err
	}
	if !reflect.DeepEqual(first.View(), second.View()) {
		return Result{}, errors.New("adapter is nondeterministic")
	}
	a, _ := json.Marshal(first)
	b, _ := json.Marshal(second)
	if string(a) != string(b) {
		return Result{}, errors.New("adapter bytes are nondeterministic")
	}
	view := first.View()
	view.Nodes[0].Name = "mutated"
	if first.Nodes()[0].Name == "mutated" {
		return Result{}, errors.New("adapter artifact is mutable")
	}
	return Result{Cases: 3}, nil
}
