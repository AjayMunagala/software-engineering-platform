package conformance

import (
	"github.com/AjayMunagala/software-engineering-platform/backend/service/repository"
)

// Scenario identifies immutable state that every adapter fixture must seed.
type Scenario struct {
	PrimaryScope  repository.Scope
	OtherScope    repository.Scope
	Repository    repository.Repository
	SucceededScan repository.Scan
	RunningScan   repository.Scan
	Artifact      repository.Artifact
	Payload       []byte
	SourceHandle  string
	Profile       repository.AnalysisProfile
}

func (scenario Scenario) clone() Scenario {
	result := scenario
	result.Payload = append([]byte(nil), scenario.Payload...)
	return result
}

// Fixture holds one isolated candidate implementation and seeded state.
type Fixture struct {
	Service  repository.Service
	Contract *repository.Contract
	Scenario Scenario
}
