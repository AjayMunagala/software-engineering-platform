// Package spike contains the isolated Phase 3.2 PostgreSQL benchmark.
// It is experimental evidence and is not a production persistence adapter.
package spike

import "context"

// GenerateFixtures emits exact deterministic JSON payloads for the approved
// released artifact fixtures.
func GenerateFixtures(ctx context.Context, config FixtureConfig) (FixtureManifest, error) {
	return generateFixtures(ctx, config)
}

// RunBenchmark executes the approved disposable PostgreSQL benchmark.
func RunBenchmark(ctx context.Context, config Config) (BenchmarkReport, error) {
	return runBenchmark(ctx, config)
}
