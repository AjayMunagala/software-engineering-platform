// Package repositoryservice contains the isolated Phase 4.0.1 design spike.
// It is experimental evidence and is not the production Repository Service API.
package repositoryservice

import (
	"context"
	"io"
)

// EncodeFunc writes one deterministic durable artifact representation.
type EncodeFunc func(context.Context, io.Writer) error

// PublicationStateReader reads the durable state after an ambiguous publish
// response. Implementations must return only service-neutral states.
type PublicationStateReader interface {
	ScanState(context.Context, string) (PublicationState, error)
}
