package framework

import "errors"

var (
	ErrLanguageRequired     = errors.New("Framework Engine requires the LanguageInventory artifact")
	ErrNoManifestNames      = errors.New("Framework Engine requires at least one manifest name")
	ErrInvalidManifestLimit = errors.New("Framework Engine maximum manifest size must be positive")
	ErrManifestTooLarge     = errors.New("manifest exceeds configured size limit")
)
