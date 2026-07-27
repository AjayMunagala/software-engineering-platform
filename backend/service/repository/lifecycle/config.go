package lifecycle

import (
	"fmt"
	"time"
)

const defaultSourceCloseTimeout = 5 * time.Second

type Config struct {
	SourceCloseTimeout time.Duration
}

func DefaultConfig() Config { return Config{SourceCloseTimeout: defaultSourceCloseTimeout} }

func (config Config) withDefaults() Config {
	if config.SourceCloseTimeout == 0 {
		config.SourceCloseTimeout = defaultSourceCloseTimeout
	}
	return config
}

func (config Config) Validate() error {
	if config.SourceCloseTimeout < time.Second || config.SourceCloseTimeout > time.Minute {
		return fmt.Errorf("%w: SourceCloseTimeout must be between 1 second and 1 minute", ErrInvalidConfig)
	}
	return nil
}
