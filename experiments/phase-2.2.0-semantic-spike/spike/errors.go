package spike

import "errors"

var (
	ErrTooManyConfigs = errors.New("at most one spike configuration is accepted")
	ErrInvalidConfig  = errors.New("invalid spike configuration")
	ErrInvalidInput   = errors.New("invalid spike input")
)
