// Package ignore implements RIE v0.2 Ignore Engine.
package ignore

import "github.com/AjayMunagala/software-engineering-platform/backend/rie"

// Engine is the public contract for repository ignore processing.
type Engine interface {
	rie.Engine
}
