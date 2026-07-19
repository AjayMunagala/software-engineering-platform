package rie

import "errors"

var (
	// ErrRunContextRequired indicates that a pipeline or engine received no context.
	ErrRunContextRequired = errors.New("RIE run context is required")
	// ErrNilEngine indicates an invalid nil stage registration.
	ErrNilEngine = errors.New("RIE pipeline cannot register a nil engine")
)
