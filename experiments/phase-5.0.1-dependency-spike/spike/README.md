# Spike Package

Purpose: validate the Dependency Intelligence graph model and algorithms before
production implementation is authorized.

Inputs: in-memory typed nodes and edges that stand in for released immutable
artifact facts.

Outputs: deterministic experimental nodes, aggregated edges, SCCs, cycles,
statistics, and bounded impact results.

Dependencies: Go standard library plus compile-time consumption of released
backend artifact types through the experiment module's local replacement.

This package performs no filesystem, network, database, process, source-parser,
or persistence operation. It is disposable evidence code and must not be
imported by `backend/`.
