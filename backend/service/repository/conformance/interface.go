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

// LifecycleFactory creates a fresh fixture for the Phase 4.0.3 lifecycle-only
// conformance suite. It does not require scan or artifact capabilities.
type LifecycleFactory interface {
	OpenLifecycle(context.Context) (LifecycleFixture, Cleanup, error)
}

type LifecycleFactoryFunc func(context.Context) (LifecycleFixture, Cleanup, error)

func (function LifecycleFactoryFunc) OpenLifecycle(ctx context.Context) (LifecycleFixture, Cleanup, error) {
	return function(ctx)
}

// ScanFactory creates a fresh fixture for the Phase 4.0.4 scan-and-artifact
// conformance suite. It does not require repository lifecycle capability.
type ScanFactory interface {
	OpenScan(context.Context) (ScanFixture, Cleanup, error)
}

type ScanFactoryFunc func(context.Context) (ScanFixture, Cleanup, error)

func (function ScanFactoryFunc) OpenScan(ctx context.Context) (ScanFixture, Cleanup, error) {
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

// RunLifecycle executes the reusable repository-lifecycle contract first,
// before any store-specific integration tests.
func RunLifecycle(t *testing.T, factory LifecycleFactory, configs ...Config) {
	t.Helper()
	suite, err := New(configs...)
	if err != nil {
		t.Fatalf("create repository lifecycle conformance suite: %v", err)
	}
	suite.RunLifecycle(t, factory)
}

// RunScan executes the reusable scan and artifact contract before any
// store-specific integration tests.
func RunScan(t *testing.T, factory ScanFactory, configs ...Config) {
	t.Helper()
	suite, err := New(configs...)
	if err != nil {
		t.Fatalf("create scan conformance suite: %v", err)
	}
	suite.RunScan(t, factory)
}
