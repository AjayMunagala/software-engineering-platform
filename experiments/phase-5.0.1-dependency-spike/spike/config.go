package spike

import "fmt"

const (
	defaultWorkers      = 1
	defaultMaxEdges     = 2_000_000
	defaultMaxEvidence  = 8
	defaultMaxTraversal = 100_000
	defaultMaxDepth     = 256
	checkpointInterval  = 1024
)

// Config bounds experimental graph work. Zero values select defaults.
type Config struct {
	Workers            int
	MaxEdges           int
	MaxEvidencePerEdge int
	MaxTraversalNodes  int
	MaxTraversalDepth  int
}

func DefaultConfig() Config {
	return Config{
		Workers: defaultWorkers, MaxEdges: defaultMaxEdges,
		MaxEvidencePerEdge: defaultMaxEvidence,
		MaxTraversalNodes:  defaultMaxTraversal, MaxTraversalDepth: defaultMaxDepth,
	}
}

func (config Config) withDefaults() Config {
	defaults := DefaultConfig()
	if config.Workers == 0 {
		config.Workers = defaults.Workers
	}
	if config.MaxEdges == 0 {
		config.MaxEdges = defaults.MaxEdges
	}
	if config.MaxEvidencePerEdge == 0 {
		config.MaxEvidencePerEdge = defaults.MaxEvidencePerEdge
	}
	if config.MaxTraversalNodes == 0 {
		config.MaxTraversalNodes = defaults.MaxTraversalNodes
	}
	if config.MaxTraversalDepth == 0 {
		config.MaxTraversalDepth = defaults.MaxTraversalDepth
	}
	return config
}

func (config Config) Validate() error {
	if config.Workers < 1 || config.Workers > 8 {
		return fmt.Errorf("%w: workers must be between 1 and 8", ErrInvalidConfig)
	}
	if config.MaxEdges < 1 || config.MaxEvidencePerEdge < 1 || config.MaxTraversalNodes < 1 || config.MaxTraversalDepth < 1 {
		return fmt.Errorf("%w: limits must be positive", ErrInvalidConfig)
	}
	return nil
}
