package spike

import "errors"

var (
	ErrInvalidConfig      = errors.New("invalid Phase 3.2 spike configuration")
	ErrFixtureIntegrity   = errors.New("fixture integrity failure")
	ErrUnsafeDatabase     = errors.New("refusing non-benchmark database")
	ErrBenchmarkIntegrity = errors.New("benchmark integrity failure")
)
