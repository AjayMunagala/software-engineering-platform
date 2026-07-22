package spike

import "context"

// Runner is the experimental full-rebuild semantic harness.
type Runner interface {
	Run(context.Context, Input) (Result, error)
}

// New creates the isolated spike runner.
func New(configs ...Config) (Runner, error) {
	config := DefaultConfig()
	if len(configs) > 1 {
		return nil, ErrTooManyConfigs
	}
	if len(configs) == 1 {
		config = configs[0].withDefaults()
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return &runner{config: config}, nil
}
