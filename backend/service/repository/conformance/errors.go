package conformance

import "errors"

var (
	ErrInvalidConfig   = errors.New("invalid repository service conformance configuration")
	ErrFactoryRequired = errors.New("repository service conformance factory is required")
	ErrInvalidFixture  = errors.New("invalid repository service conformance fixture")
)
