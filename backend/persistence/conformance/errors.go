package conformance

import "errors"

var (
	ErrInvalidConfig   = errors.New("invalid conformance configuration")
	ErrFactoryRequired = errors.New("conformance factory is required")
	ErrFixtureInvalid  = errors.New("conformance fixture is invalid")
)
