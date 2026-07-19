// Package discovery implements RIE v0.1 Discovery Engine.
package discovery

import "github.com/AjayMunagala/software-engineering-platform/backend/rie"

// Engine is the public contract for repository discovery.
type Engine interface {
	rie.Engine
	Scan(repositoryPath string) (Report, error)
}
