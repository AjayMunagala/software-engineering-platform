package observability

import (
	"slices"
	"time"

	runtimehealth "github.com/AjayMunagala/software-engineering-platform/backend/internal/runtime/health"
)

type Level string

const (
	LevelDebug Level = "debug"
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

type Format string

const (
	FormatText Format = "text"
	FormatJSON Format = "json"
)

type Component string

const (
	ComponentRuntime       Component = "runtime"
	ComponentConfiguration Component = "configuration"
	ComponentPostgreSQL    Component = "postgresql"
	ComponentHealth        Component = "health"
	ComponentObservability Component = "observability"
)

type Outcome string

const (
	OutcomeSuccess  Outcome = "success"
	OutcomeError    Outcome = "error"
	OutcomeTimeout  Outcome = "timeout"
	OutcomeCanceled Outcome = "canceled"
	OutcomeForced   Outcome = "forced"
)

type EventName string

const (
	EventStartup     EventName = "startup"
	EventReady       EventName = "ready"
	EventDraining    EventName = "draining"
	EventStopping    EventName = "stopping"
	EventStopped     EventName = "stopped"
	EventHealthLost  EventName = "health_lost"
	EventHealthReady EventName = "health_recovered"
	EventExport      EventName = "metrics_export"
)

// EventParams deliberately contains no arbitrary map or raw error. This is
// the redaction boundary for runtime structured logging.
type EventParams struct {
	Level         Level
	Component     Component
	Event         EventName
	Outcome       Outcome
	Duration      time.Duration
	CorrelationID string
	ErrorKind     string
	RuntimeState  runtimehealth.RuntimeState
	Count         int64
}

type PoolSnapshot struct {
	name                         string
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

type PoolSnapshotParams struct {
	Name                         string
	Acquired                     int32
	Idle                         int32
	Constructing                 int32
	Total                        int32
	Maximum                      int32
	AcquireCount                 int64
	AcquireDuration              time.Duration
	EmptyAcquireCount            int64
	EmptyAcquireWait             time.Duration
	ConnectionsCreated           int64
	ConnectionsDestroyedIdle     int64
	ConnectionsDestroyedLifetime int64
}

func (value PoolSnapshot) Name() string                    { return value.name }
func (value PoolSnapshot) Acquired() int32                 { return value.acquired }
func (value PoolSnapshot) Idle() int32                     { return value.idle }
func (value PoolSnapshot) Constructing() int32             { return value.constructing }
func (value PoolSnapshot) Total() int32                    { return value.total }
func (value PoolSnapshot) Maximum() int32                  { return value.maximum }
func (value PoolSnapshot) AcquireCount() int64             { return value.acquireCount }
func (value PoolSnapshot) AcquireDuration() time.Duration  { return value.acquireDuration }
func (value PoolSnapshot) EmptyAcquireCount() int64        { return value.emptyAcquireCount }
func (value PoolSnapshot) EmptyAcquireWait() time.Duration { return value.emptyAcquireWait }
func (value PoolSnapshot) ConnectionsCreated() int64       { return value.connectionsCreated }
func (value PoolSnapshot) ConnectionsDestroyedIdle() int64 { return value.connectionsDestroyedIdle }
func (value PoolSnapshot) ConnectionsDestroyedLifetime() int64 {
	return value.connectionsDestroyedLifetime
}

// RuntimeSnapshot is a detached, low-cardinality runtime view.
type RuntimeSnapshot struct {
	observedAt       time.Time
	state            runtimehealth.RuntimeState
	liveness         runtimehealth.Status
	readiness        runtimehealth.Status
	inFlight         int
	schemaCompatible bool
	healthDuration   time.Duration
	pools            []PoolSnapshot
}

type RuntimeSnapshotParams struct {
	ObservedAt       time.Time
	State            runtimehealth.RuntimeState
	Liveness         runtimehealth.Status
	Readiness        runtimehealth.Status
	InFlight         int
	SchemaCompatible bool
	HealthDuration   time.Duration
	Pools            []PoolSnapshot
}

func (value RuntimeSnapshot) ObservedAt() time.Time             { return value.observedAt }
func (value RuntimeSnapshot) State() runtimehealth.RuntimeState { return value.state }
func (value RuntimeSnapshot) Liveness() runtimehealth.Status    { return value.liveness }
func (value RuntimeSnapshot) Readiness() runtimehealth.Status   { return value.readiness }
func (value RuntimeSnapshot) InFlight() int                     { return value.inFlight }
func (value RuntimeSnapshot) SchemaCompatible() bool            { return value.schemaCompatible }
func (value RuntimeSnapshot) HealthDuration() time.Duration     { return value.healthDuration }
func (value RuntimeSnapshot) Pools() []PoolSnapshot             { return slices.Clone(value.pools) }

type MetricType string

const (
	MetricGauge     MetricType = "gauge"
	MetricCounter   MetricType = "counter"
	MetricHistogram MetricType = "histogram_observation"
)

type Label struct {
	name  string
	value string
}

func (value Label) Name() string  { return value.name }
func (value Label) Value() string { return value.value }

type Metric struct {
	name   string
	kind   MetricType
	value  float64
	labels []Label
}

func (value Metric) Name() string     { return value.name }
func (value Metric) Type() MetricType { return value.kind }
func (value Metric) Value() float64   { return value.value }
func (value Metric) Labels() []Label  { return slices.Clone(value.labels) }

type MetricSnapshot struct {
	observedAt time.Time
	metrics    []Metric
}

func (value MetricSnapshot) ObservedAt() time.Time { return value.observedAt }
func (value MetricSnapshot) Metrics() []Metric {
	result := slices.Clone(value.metrics)
	for index := range result {
		result[index].labels = slices.Clone(result[index].labels)
	}
	return result
}

type Statistics struct {
	collections uint64
	exports     uint64
	failures    uint64
	dropped     uint64
}

func (value Statistics) Collections() uint64 { return value.collections }
func (value Statistics) Exports() uint64     { return value.exports }
func (value Statistics) Failures() uint64    { return value.failures }
func (value Statistics) Dropped() uint64     { return value.dropped }
