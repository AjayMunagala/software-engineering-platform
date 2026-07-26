package observability

import (
	"io"
	"time"
)

const (
	maximumServiceNameBytes = 64
	minimumInterval         = time.Second
	maximumInterval         = time.Hour
	defaultExportTimeout    = 2 * time.Second
)

type Config struct {
	serviceName    string
	level          Level
	format         Format
	metricsEnabled bool
	interval       time.Duration
	exportTimeout  time.Duration
}

type ConfigParams struct {
	ServiceName    string
	Level          Level
	Format         Format
	MetricsEnabled bool
	Interval       time.Duration
	ExportTimeout  time.Duration
}

func (value Config) ServiceName() string          { return value.serviceName }
func (value Config) Level() Level                 { return value.level }
func (value Config) Format() Format               { return value.format }
func (value Config) MetricsEnabled() bool         { return value.metricsEnabled }
func (value Config) Interval() time.Duration      { return value.interval }
func (value Config) ExportTimeout() time.Duration { return value.exportTimeout }

// NewConfig validates the bounded telemetry policy.
func NewConfig(params ConfigParams) (Config, error) {
	if !safeIdentifier(params.ServiceName, maximumServiceNameBytes) || !validLevel(params.Level) || !validFormat(params.Format) {
		return Config{}, newError(ErrorInvalidInput, "config", nil)
	}
	if params.Interval < minimumInterval || params.Interval > maximumInterval {
		return Config{}, newError(ErrorInvalidInput, "collection-interval", nil)
	}
	if params.ExportTimeout == 0 {
		params.ExportTimeout = defaultExportTimeout
		if params.ExportTimeout > params.Interval {
			params.ExportTimeout = params.Interval
		}
	}
	if params.ExportTimeout < time.Millisecond || params.ExportTimeout > params.Interval {
		return Config{}, newError(ErrorInvalidInput, "export-timeout", nil)
	}
	return Config{params.ServiceName, params.Level, params.Format, params.MetricsEnabled, params.Interval, params.ExportTimeout}, nil
}

// NewService creates a runtime-owned logger and optional metrics collector.
func NewService(config Config, output io.Writer, sink Sink) (Service, error) {
	return newService(config, output, sink)
}

func validLevel(value Level) bool {
	return value == LevelDebug || value == LevelInfo || value == LevelWarn || value == LevelError
}

func validFormat(value Format) bool { return value == FormatText || value == FormatJSON }
