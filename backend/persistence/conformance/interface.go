// Package conformance provides adapter-independent persistence contract tests.
package conformance

import (
	"context"
	"testing"
)

// Cleanup releases one isolated fixture. It must be idempotent.
type Cleanup func(context.Context) error

// Factory creates a pre-seeded isolated fixture for one conformance subtest.
// The primary repository must have one succeeded scan and one published exact
// artifact. OtherScope must not be authorized for Primary.
type Factory interface {
	Open(context.Context) (Fixture, Cleanup, error)
}

// FactoryFunc adapts a function to Factory.
type FactoryFunc func(context.Context) (Fixture, Cleanup, error)

func (function FactoryFunc) Open(ctx context.Context) (Fixture, Cleanup, error) {
	return function(ctx)
}

// Run executes every currently accepted conformance requirement.
func Run(t *testing.T, factory Factory, configs ...Config) {
	t.Helper()
	suite, err := New(configs...)
	if err != nil {
		t.Fatalf("create persistence conformance suite: %v", err)
	}
	suite.Run(t, factory)
}
