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

type LifecycleFixture struct {
	Service  repository.RepositoryLifecycleService
	Contract *repository.Contract
	Scenario LifecycleScenario
}

type LifecycleScenario struct {
	PrimaryScope repository.Scope
	OtherScope   repository.Scope
	Repository   repository.Repository
	SourceHandle string
}

type ScanService interface {
	repository.ScanExecutionService
	repository.ArtifactQueryService
}

type ScanFixture struct {
	Service  ScanService
	Contract *repository.Contract
	Scenario ScanScenario
}

type ScanScenario struct {
	PrimaryScope  repository.Scope
	OtherScope    repository.Scope
	RepositoryID  repository.RepositoryID
	SucceededScan repository.Scan
	RunningScan   repository.Scan
	Artifact      repository.Artifact
	Payload       []byte
	SourceHandle  string
	Profile       repository.AnalysisProfile
}

func (scenario ScanScenario) clone() ScanScenario {
	result := scenario
	result.Payload = append([]byte(nil), scenario.Payload...)
	return result
}

func (scenario LifecycleScenario) clone() LifecycleScenario { return scenario }

func (scenario Scenario) clone() Scenario {
	result := scenario
	result.Payload = append([]byte(nil), scenario.Payload...)
	return result
}

// Fixture holds one isolated conforming implementation and seeded state.
type Fixture struct {
	Service  repository.Service
	Contract *repository.Contract
	Scenario Scenario
}
