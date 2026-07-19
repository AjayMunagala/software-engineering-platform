package discovery

import "errors"

var (
	ErrRepositoryPathRequired = errors.New("repository path is required")
	ErrRepositoryNotDirectory = errors.New("repository path is not a directory")
)
