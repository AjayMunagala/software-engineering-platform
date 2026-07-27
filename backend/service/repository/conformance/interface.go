// Package conformance provides adapter-independent Repository Service tests.
package conformance

import (
	"context"
	"testing"
)

// Cleanup releases one isolated fixture and must be idempotent.
type Cleanup func(context.Context) error

// Factory creates a fresh, pre-seeded fixture for one conformance subtest.
type Factory interface {
	Open(context.Context) (Fixture, Cleanup, error)
}

type FactoryFunc func(context.Context) (Fixture, Cleanup, error)

func (function FactoryFunc) Open(ctx context.Context) (Fixture, Cleanup, error) {
	return function(ctx)
}

// Run executes the complete candidate conformance suite.
func Run(t *testing.T, factory Factory, configs ...Config) {
	t.Helper()
	suite, err := New(configs...)
	if err != nil {
		t.Fatalf("create repository service conformance suite: %v", err)
	}
	suite.Run(t, factory)
}
