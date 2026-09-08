package conformance

import (
	"context"
	"errors"
	"testing"

	"github.com/AjayMunagala/software-engineering-platform/backend/die"
)

type failingCore struct{ err error }

func (f failingCore) Name() string    { return "fail" }
func (f failingCore) Version() string { return "0" }
func (f failingCore) Normalize(context.Context, die.GraphInput) (die.DependencyInventory, error) {
	return die.DependencyInventory{}, f.err
}

type changingCore struct {
	base  die.Core
	calls int
}

func (c *changingCore) Name() string    { return "change" }
func (c *changingCore) Version() string { return "0" }
func (c *changingCore) Normalize(ctx context.Context, in die.GraphInput) (die.DependencyInventory, error) {
	c.calls++
	if c.calls == 1 {
		return c.base.Normalize(ctx, in)
	}
	return c.base.Normalize(ctx, die.GraphInput{})
}

func TestReferenceCore(t *testing.T) {
	result, err := Run(t, Config{Factory: die.New})
	if err != nil {
		t.Fatal(err)
	}
	if result.Cases != 3 {
		t.Fatalf("cases=%d", result.Cases)
	}
}

func TestMissingFactory(t *testing.T) {
	if _, err := Run(t, Config{}); err != ErrMissingFactory {
		t.Fatalf("got %v", err)
	}
}
func TestFactoryFailure(t *testing.T) {
	if _, err := Run(t, Config{Factory: func(die.Config) (die.Core, error) { return nil, ErrMissingFactory }}); err != ErrMissingFactory {
		t.Fatalf("got %v", err)
	}
}

func TestNormalizeFailure(t *testing.T) {
	sentinel := errors.New("failure")
	_, err := Run(t, Config{Factory: func(die.Config) (die.Core, error) { return failingCore{sentinel}, nil }})
	if !errors.Is(err, sentinel) {
		t.Fatalf("got %v", err)
	}
}

func TestNondeterministicFailure(t *testing.T) {
	_, err := Run(t, Config{Factory: func(config die.Config) (die.Core, error) {
		base, buildErr := die.New(config)
		return &changingCore{base: base}, buildErr
	}})
	if err == nil {
		t.Fatal("expected nondeterminism error")
	}
}
