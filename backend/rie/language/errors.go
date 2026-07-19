package language

import "errors"

var (
	ErrIgnoreRequired      = errors.New("Language Engine requires Ignore Engine output")
	ErrNoExtensionMappings = errors.New("Language Engine requires at least one extension mapping")
)
