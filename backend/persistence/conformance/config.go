package conformance

import (
	"fmt"
	"time"
)

const defaultCaseTimeout = 10 * time.Second

// Config bounds each adapter-independent case. Database fixture creation may
// have its own external test timeout.
type Config struct {
	CaseTimeout time.Duration
}

func DefaultConfig() Config { return Config{CaseTimeout: defaultCaseTimeout} }

func (config Config) withDefaults() Config {
	if config.CaseTimeout == 0 {
		config.CaseTimeout = defaultCaseTimeout
	}
	return config
}

func (config Config) Validate() error {
	if config.CaseTimeout < time.Second || config.CaseTimeout > time.Minute {
		return fmt.Errorf("%w: CaseTimeout must be between one second and one minute", ErrInvalidConfig)
	}
	return nil
}
