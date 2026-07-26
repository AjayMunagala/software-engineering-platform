package health

import "time"

const (
	minimumTimeout  = time.Millisecond
	maximumTimeout  = time.Minute
	minimumInterval = time.Millisecond
	maximumInterval = 10 * time.Minute
)

// Clock supports deterministic health snapshots and tests.
type Clock interface{ Now() time.Time }

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

// ConfigParams is mutable caller input used only during construction.
type ConfigParams struct {
	CheckTimeout     time.Duration
	CheckInterval    time.Duration
	FailureThreshold int
	Clock            Clock
}

// Config is immutable after validation.
type Config struct {
	checkTimeout     time.Duration
	checkInterval    time.Duration
	failureThreshold int
	clock            Clock
}

func NewConfig(params ConfigParams) (Config, error) {
	if params.Clock == nil {
		params.Clock = systemClock{}
	}
	if params.CheckTimeout < minimumTimeout || params.CheckTimeout > maximumTimeout ||
		params.CheckInterval < minimumInterval || params.CheckInterval > maximumInterval ||
		params.FailureThreshold < 1 || params.FailureThreshold > 100 {
		return Config{}, newError(ErrorInvalidConfig, "health-config", nil)
	}
	return Config{
		checkTimeout: params.CheckTimeout, checkInterval: params.CheckInterval,
		failureThreshold: params.FailureThreshold, clock: params.Clock,
	}, nil
}

func (config Config) CheckTimeout() time.Duration  { return config.checkTimeout }
func (config Config) CheckInterval() time.Duration { return config.checkInterval }
func (config Config) FailureThreshold() int        { return config.failureThreshold }
