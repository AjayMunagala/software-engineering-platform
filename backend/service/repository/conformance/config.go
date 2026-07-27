package conformance

import "fmt"

const defaultMaxExportBytes = 8 << 20

type Config struct {
	MaxExportBytes int
}

func DefaultConfig() Config { return Config{MaxExportBytes: defaultMaxExportBytes} }

func (config Config) withDefaults() Config {
	if config.MaxExportBytes == 0 {
		config.MaxExportBytes = defaultMaxExportBytes
	}
	return config
}

func (config Config) Validate() error {
	if config.MaxExportBytes < 1 || config.MaxExportBytes > defaultMaxExportBytes {
		return fmt.Errorf("%w: MaxExportBytes must be between 1 and %d", ErrInvalidConfig, defaultMaxExportBytes)
	}
	return nil
}
