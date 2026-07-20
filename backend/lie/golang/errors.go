package golang

import "errors"

var (
	ErrSourceMissing     = errors.New("Go source file is missing")
	ErrSourceUnreadable  = errors.New("Go source file is unreadable")
	ErrSourceOversized   = errors.New("Go source file exceeds configured size limit")
	ErrSourceOutsideRoot = errors.New("Go source path is outside the repository root")
)
