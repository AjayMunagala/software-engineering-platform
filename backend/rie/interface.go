// Package rie contains contracts shared by Repository Intelligence Engine stages.
package rie

import "context"

// Engine is the common execution contract for every RIE pipeline stage.
// Engines are self-describing, independently versioned, and operate only on
// the shared RunContext passed to Execute.
type Engine interface {
	Name() string
	Version() string
	Description() string
	Execute(context.Context, *RunContext) error
}
