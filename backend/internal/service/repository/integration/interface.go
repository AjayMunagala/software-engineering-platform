// Package integration connects the Repository Service coordinators to the
// frozen persistence and runtime contracts. It owns translations only.
package integration

import (
	"context"

	runtimeapp "github.com/AjayMunagala/software-engineering-platform/backend/internal/runtime/app"
	runtimepostgres "github.com/AjayMunagala/software-engineering-platform/backend/internal/runtime/postgres"
	serviceadapters "github.com/AjayMunagala/software-engineering-platform/backend/internal/service/repository/adapters"
	"github.com/AjayMunagala/software-engineering-platform/backend/persistence"
	"github.com/AjayMunagala/software-engineering-platform/backend/service/repository"
	"github.com/AjayMunagala/software-engineering-platform/backend/service/repository/lifecycle"
	"github.com/AjayMunagala/software-engineering-platform/backend/service/repository/scan"
)

const ContractVersion = "1.0.0"

type Runtime interface {
	Admit(context.Context) (runtimeapp.Work, error)
	Ingest() runtimepostgres.IngestCapabilities
	Read() runtimepostgres.ReadCapabilities
}

type RepositoryPersistence interface{ persistence.RepositoryStore }

type ScanPersistence interface {
	persistence.ScanStore
	persistence.PayloadStager
	persistence.PublicationStore
	persistence.ArtifactReader
}

type Dependencies struct {
	Runtime         Runtime
	Persistence     *persistence.Contract
	ServiceContract *repository.Contract
	SourceResolver  serviceadapters.SourceResolver
	Clock           scan.Clock
}

var _ lifecycle.Store = (*Store)(nil)
var _ scan.Store = (*Store)(nil)
var _ scan.AdmissionController = (*Admission)(nil)
var _ lifecycle.SourceProofResolver = (*SourceProofAdapter)(nil)
