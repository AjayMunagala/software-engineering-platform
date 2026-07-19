package summary

import "errors"

var (
	ErrMetadataRequired  = errors.New("Repository Intelligence Summary requires RepositoryMetadata")
	ErrInvalidCapability = errors.New("Repository Intelligence Summary capability IDs must be unique and non-empty")
)
