package spike

import "errors"

var (
	ErrInvalidConfig  = errors.New("invalid dependency spike configuration")
	ErrInvalidInput   = errors.New("invalid dependency spike input")
	ErrTooManyConfigs = errors.New("only one dependency spike configuration is allowed")
	ErrNodeNotFound   = errors.New("dependency node not found")
)
