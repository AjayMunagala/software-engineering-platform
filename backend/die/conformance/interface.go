package conformance

import "github.com/AjayMunagala/software-engineering-platform/backend/die"

type Factory func(die.Config) (die.Core, error)
