package postgres

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	storagepostgres "github.com/AjayMunagala/software-engineering-platform/backend/internal/storage/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
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

// PoolStatistics is a detached, bounded operational view. It exposes no host,
// database, user, SQL, credentials, or driver object.
type PoolStatistics struct {
	capability                   Capability
	acquired                     int32
	idle                         int32
	constructing                 int32
	total                        int32
	maximum                      int32
	acquireCount                 int64
	acquireDuration              time.Duration
	emptyAcquireCount            int64
	emptyAcquireWait             time.Duration
	connectionsCreated           int64
	connectionsDestroyedIdle     int64
	connectionsDestroyedLifetime int64
}

func (value PoolStatistics) Capability() Capability          { return value.capability }
func (value PoolStatistics) Acquired() int32                 { return value.acquired }
func (value PoolStatistics) Idle() int32                     { return value.idle }
func (value PoolStatistics) Constructing() int32             { return value.constructing }
func (value PoolStatistics) Total() int32                    { return value.total }
func (value PoolStatistics) Maximum() int32                  { return value.maximum }
func (value PoolStatistics) AcquireCount() int64             { return value.acquireCount }
func (value PoolStatistics) AcquireDuration() time.Duration  { return value.acquireDuration }
func (value PoolStatistics) EmptyAcquireCount() int64        { return value.emptyAcquireCount }
func (value PoolStatistics) EmptyAcquireWait() time.Duration { return value.emptyAcquireWait }
func (value PoolStatistics) ConnectionsCreated() int64       { return value.connectionsCreated }
func (value PoolStatistics) ConnectionsDestroyedIdle() int64 { return value.connectionsDestroyedIdle }
func (value PoolStatistics) ConnectionsDestroyedLifetime() int64 {
	return value.connectionsDestroyedLifetime
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

// Statistics returns detached pool counters for runtime observability. Pools
// without a statistics capability are omitted, which keeps test adapters and
// future database implementations independent from pgx.
func (runtime *Runtime) Statistics() []PoolStatistics {
	if runtime == nil {
		return nil
	}
	result := make([]PoolStatistics, 0, len(runtime.pools))
	for _, owned := range runtime.pools {
		provider, ok := owned.pool.(interface{ Stat() *pgxpool.Stat })
		if !ok {
			continue
		}
		statistics := provider.Stat()
		if statistics == nil {
			continue
		}
		result = append(result, PoolStatistics{
			capability: owned.capability, acquired: statistics.AcquiredConns(), idle: statistics.IdleConns(),
			constructing: statistics.ConstructingConns(), total: statistics.TotalConns(), maximum: statistics.MaxConns(),
			acquireCount: statistics.AcquireCount(), acquireDuration: statistics.AcquireDuration(),
			emptyAcquireCount: statistics.EmptyAcquireCount(), emptyAcquireWait: statistics.EmptyAcquireWaitTime(),
			connectionsCreated: statistics.NewConnsCount(), connectionsDestroyedIdle: statistics.MaxIdleDestroyCount(),
			connectionsDestroyedLifetime: statistics.MaxLifetimeDestroyCount(),
		})
	}
	return result
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
