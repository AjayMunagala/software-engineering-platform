package spike

import "context"

// Runner validates the proposed deterministic graph pipeline.
type Runner interface {
	Run(context.Context, Input) (Result, error)
	Impact(context.Context, Result, ImpactQuery) (ImpactResult, error)
}

// New constructs an isolated spike runner.
func New(configs ...Config) (Runner, error) {
	if len(configs) > 1 {
		return nil, ErrTooManyConfigs
	}
	config := DefaultConfig()
	if len(configs) == 1 {
		config = configs[0].withDefaults()
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return &runner{config: config}, nil
}
