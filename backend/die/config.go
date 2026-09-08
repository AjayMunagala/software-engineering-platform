package die

import "fmt"

const (
	DefaultMaxNodes           uint64 = 1_000_000
	DefaultMaxEdges           uint64 = 2_000_000
	DefaultMaxEvidencePerItem uint64 = 8
	DefaultMaxDiagnostics     uint64 = 1_000
	MaximumMaxNodes           uint64 = 10_000_000
	MaximumMaxEdges           uint64 = 20_000_000
	MaximumMaxEvidencePerItem uint64 = 1_024
	MaximumMaxDiagnostics     uint64 = 1_000_000
)

type ConfigParams struct {
	MaxNodes           uint64
	MaxEdges           uint64
	MaxEvidencePerItem uint64
	MaxDiagnostics     uint64
}

type Config struct {
	maxNodes, maxEdges, maxEvidencePerItem, maxDiagnostics uint64
}

func NewConfig(params ConfigParams) (Config, error) {
	if params.MaxNodes == 0 {
		params.MaxNodes = DefaultMaxNodes
	}
	if params.MaxEdges == 0 {
		params.MaxEdges = DefaultMaxEdges
	}
	if params.MaxEvidencePerItem == 0 {
		params.MaxEvidencePerItem = DefaultMaxEvidencePerItem
	}
	if params.MaxDiagnostics == 0 {
		params.MaxDiagnostics = DefaultMaxDiagnostics
	}
	if params.MaxNodes > MaximumMaxNodes {
		return Config{}, newError(ErrorInvalidInput, "invalid_max_nodes", fmt.Sprintf("MaxNodes must not exceed %d", MaximumMaxNodes), nil)
	}
	if params.MaxEdges > MaximumMaxEdges {
		return Config{}, newError(ErrorInvalidInput, "invalid_max_edges", fmt.Sprintf("MaxEdges must not exceed %d", MaximumMaxEdges), nil)
	}
	if params.MaxEvidencePerItem > MaximumMaxEvidencePerItem {
		return Config{}, newError(ErrorInvalidInput, "invalid_max_evidence", fmt.Sprintf("MaxEvidencePerItem must not exceed %d", MaximumMaxEvidencePerItem), nil)
	}
	if params.MaxDiagnostics > MaximumMaxDiagnostics {
		return Config{}, newError(ErrorInvalidInput, "invalid_max_diagnostics", fmt.Sprintf("MaxDiagnostics must not exceed %d", MaximumMaxDiagnostics), nil)
	}
	return Config{params.MaxNodes, params.MaxEdges, params.MaxEvidencePerItem, params.MaxDiagnostics}, nil
}

func DefaultConfig() Config                 { config, _ := NewConfig(ConfigParams{}); return config }
func (c Config) MaxNodes() uint64           { return c.maxNodes }
func (c Config) MaxEdges() uint64           { return c.maxEdges }
func (c Config) MaxEvidencePerItem() uint64 { return c.maxEvidencePerItem }
func (c Config) MaxDiagnostics() uint64     { return c.maxDiagnostics }
