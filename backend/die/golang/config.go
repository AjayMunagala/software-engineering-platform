package golang

import "github.com/AjayMunagala/software-engineering-platform/backend/die"

const (
	DefaultMaxInputRecords  uint64 = 2_000_000
	DefaultMaxInputEvidence uint64 = 4_000_000
	MaximumInputRecords     uint64 = 10_000_000
	MaximumInputEvidence    uint64 = 20_000_000
)

type ConfigParams struct {
	CoreConfig                        die.Config
	MaxInputRecords, MaxInputEvidence uint64
}
type Config struct {
	core              die.Config
	records, evidence uint64
	valid             bool
}

func NewConfig(p ConfigParams) (Config, error) {
	if p.CoreConfig == (die.Config{}) {
		p.CoreConfig = die.DefaultConfig()
	}
	if _, err := die.New(p.CoreConfig); err != nil {
		return Config{}, fail(die.ErrorInvalidInput, "invalid_config")
	}
	if p.MaxInputRecords == 0 {
		p.MaxInputRecords = DefaultMaxInputRecords
	}
	if p.MaxInputEvidence == 0 {
		p.MaxInputEvidence = DefaultMaxInputEvidence
	}
	if p.MaxInputRecords > MaximumInputRecords || p.MaxInputEvidence > MaximumInputEvidence {
		return Config{}, fail(die.ErrorInvalidInput, "invalid_config")
	}
	return Config{p.CoreConfig, p.MaxInputRecords, p.MaxInputEvidence, true}, nil
}
func (c Config) CoreConfig() die.Config   { return c.core }
func (c Config) MaxInputRecords() uint64  { return c.records }
func (c Config) MaxInputEvidence() uint64 { return c.evidence }
