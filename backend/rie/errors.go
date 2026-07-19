package rie

import "errors"

var (
	// ErrRunContextRequired indicates that a pipeline or engine received no context.
	ErrRunContextRequired = errors.New("RIE run context is required")
	// ErrNilEngine indicates an invalid nil stage registration.
	ErrNilEngine = errors.New("RIE pipeline cannot register a nil engine")
	// ErrNilArtifact indicates an invalid nil artifact publication.
	ErrNilArtifact = errors.New("RIE artifact cannot be nil")
	// ErrArtifactNameRequired indicates an artifact without a stable identity.
	ErrArtifactNameRequired = errors.New("RIE artifact name is required")
	// ErrArtifactAlreadyExists prevents one engine from silently replacing another artifact.
	ErrArtifactAlreadyExists = errors.New("RIE artifact already exists")
)
