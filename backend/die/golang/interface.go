// Package golang translates released Go artifacts into neutral dependency facts.
package golang

import (
	"context"
	"github.com/AjayMunagala/software-engineering-platform/backend/die"
)

const Version = "0.1.0"
const BoundaryNameScheme = "go-dependency-boundary-name/v1"

type Engine interface {
	Name() string
	Version() string
	Description() string
	Analyze(context.Context, Inputs) (die.DependencyInventory, error)
}

func New(config Config) (Engine, error) {
	if !config.valid {
		return nil, fail(die.ErrorInvalidInput, "invalid_config")
	}
	core, err := die.New(config.core)
	if err != nil {
		return nil, fail(die.ErrorInvalidInput, "invalid_config")
	}
	return &engine{config: config, core: core}, nil
}
