package postgres

import (
	"errors"
	"time"
)

const ChunkSize = 4 << 20

var ErrInvalidConfig = errors.New("invalid PostgreSQL persistence adapter configuration")

// Clock makes lifecycle timestamps deterministic in tests.
type Clock interface{ Now() time.Time }

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

// Config contains adapter-local behavior only. Payload size and publication
// limits remain owned by the neutral contract and frozen database schema.
type Config struct{ Clock Clock }

func DefaultConfig() Config { return Config{Clock: systemClock{}} }

func (config Config) withDefaults() Config {
	if config.Clock == nil {
		config.Clock = systemClock{}
	}
	return config
}

func (config Config) Validate() error {
	if config.Clock == nil {
		return ErrInvalidConfig
	}
	return nil
}
