package die

import "context"

const (
	ContractVersion = "0.1.0"
	ArtifactName    = "dependency-inventory"
	ArtifactVersion = "0.1.0"
	EngineName      = "dependency-intelligence-core"
	EngineVersion   = "0.1.0"
)

type Core interface {
	Name() string
	Version() string
	Normalize(context.Context, GraphInput) (DependencyInventory, error)
}

func New(config Config) (Core, error) {
	if config.MaxNodes() == 0 || config.MaxEdges() == 0 || config.MaxEvidencePerItem() == 0 || config.MaxDiagnostics() == 0 {
		return nil, newError(ErrorInvalidInput, "invalid_config", "configuration must be created by NewConfig", nil)
	}
	return &core{config: config}, nil
}
