package postgres

import "time"

const (
	CompatibilityContractKey = "aegis-postgresql-persistence"
	SchemaContractVersion    = "1.0.0"
	AdapterMajor             = 1
	ExpectedPostgreSQLMajor  = 18
	compatibilityRelation    = "platform.runtime_compatibility"
	maximumTLSMaterialBytes  = 1 << 20
)

type clock interface{ Now() time.Time }

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }
