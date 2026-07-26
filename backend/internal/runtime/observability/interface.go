// Package observability provides transport-neutral structured runtime events
// and bounded metric snapshots. It owns no listener, exporter protocol,
// database connection, credential, SQL, or product data.
package observability

import "context"

// ContractVersion identifies the frozen Runtime Observability contract.
const ContractVersion = "1.0.0"

// Source produces one detached, bounded runtime snapshot.
type Source interface {
	ObservabilitySnapshot(context.Context) RuntimeSnapshot
}

// Sink exports one detached metric snapshot. Implementations must honor the
// supplied context and must not retain mutable caller-owned state.
type Sink interface {
	Export(context.Context, MetricSnapshot) error
}

// Service is the narrow capability consumed by lifecycle orchestration.
type Service interface {
	Event(context.Context, EventParams) error
	Start(context.Context, Source) error
	StopCollection(context.Context) error
	Close(context.Context) error
	Statistics() Statistics
}
