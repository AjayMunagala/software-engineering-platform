package postgres

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	storagepostgres "github.com/AjayMunagala/software-engineering-platform/backend/internal/storage/postgres"
)

// CompatibilityProof is the immutable migration-maintained contract accepted
// during startup.
type CompatibilityProof struct {
	contractKey         string
	schemaVersion       string
	minimumAdapterMajor int
	maximumAdapterMajor int
	migrationRevision   string
	publishedAt         time.Time
}

func (proof CompatibilityProof) ContractKey() string       { return proof.contractKey }
func (proof CompatibilityProof) SchemaVersion() string     { return proof.schemaVersion }
func (proof CompatibilityProof) MinimumAdapterMajor() int  { return proof.minimumAdapterMajor }
func (proof CompatibilityProof) MaximumAdapterMajor() int  { return proof.maximumAdapterMajor }
func (proof CompatibilityProof) MigrationRevision() string { return proof.migrationRevision }
func (proof CompatibilityProof) PublishedAt() time.Time    { return proof.publishedAt }

type ownedPool struct {
	capability Capability
	pool       databasePool
}

// Runtime owns all created pools and exposes only narrow persistence
// capabilities. It does not own application health or work admission.
type Runtime struct {
	ingest    IngestCapabilities
	read      ReadCapabilities
	retention RetentionCapabilities
	proof     CompatibilityProof
	pools     []ownedPool
	checks    []func(context.Context) error
	closed    atomic.Bool
	closeOnce sync.Once
}

// Capability views deliberately prevent callers from recovering the complete
// adapter through a type assertion. Database privileges remain the final
// enforcement boundary, while the Go surface also stays least-capability.
type ingestView struct{ IngestCapabilities }
type readView struct{ ReadCapabilities }
type retentionView struct{ RetentionCapabilities }

func (runtime *Runtime) Ingest() IngestCapabilities {
	if runtime == nil {
		return nil
	}
	return runtime.ingest
}

func (runtime *Runtime) Read() ReadCapabilities {
	if runtime == nil {
		return nil
	}
	return runtime.read
}

func (runtime *Runtime) Retention() RetentionCapabilities {
	if runtime == nil {
		return nil
	}
	return runtime.retention
}

func (runtime *Runtime) Compatibility() CompatibilityProof {
	if runtime == nil {
		return CompatibilityProof{}
	}
	return runtime.proof
}

// PoolCount returns only the number of uniquely owned pools.
func (runtime *Runtime) PoolCount() int {
	if runtime == nil {
		return 0
	}
	return len(runtime.pools)
}

// Check verifies every required PostgreSQL capability through the same
// read-only startup proof. Pool internals remain opaque to health callers.
func (runtime *Runtime) Check(ctx context.Context) error {
	if runtime == nil || ctx == nil {
		return newError(ErrorInvalidInput, "runtime-check", "", nil)
	}
	if err := ctx.Err(); err != nil {
		return newError(ErrorCanceled, "runtime-check", "", err)
	}
	if runtime.closed.Load() {
		return newError(ErrorUnavailable, "runtime-closed", "", nil)
	}
	for _, check := range runtime.checks {
		if err := check(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Close idempotently closes unique pools in reverse construction order.
func (runtime *Runtime) Close() {
	if runtime == nil {
		return
	}
	runtime.closeOnce.Do(func() {
		runtime.closed.Store(true)
		for index := len(runtime.pools) - 1; index >= 0; index-- {
			runtime.pools[index].pool.Close()
		}
	})
}

func adapterCapabilities(adapter *storagepostgres.Adapter) (IngestCapabilities, ReadCapabilities, RetentionCapabilities) {
	return ingestView{adapter}, readView{adapter}, retentionView{adapter}
}
