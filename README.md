# Aegis CodeMind

Working name for an evidence-first AI software engineering platform.

## Purpose

Help engineers understand codebases, investigate failures, identify root causes, propose the smallest safe patch, and validate it with evidence.

This is not a general-purpose chatbot. The first release is deliberately narrow: local repository intelligence.

## Status

Repository Intelligence Engine **1.0.0** is implemented and frozen. It deterministically discovers a local repository, applies ignore rules, detects languages/frameworks/build tooling, synthesizes repository metadata, and publishes an immutable executive summary. It does not use an LLM or execute repository code.

Real-world validation passed on 15 pinned public repositories across the supported ecosystems. No RIE 1.0.1 bug-fix release was required.

Go Language Intelligence **1.0.0** is also implemented, validated, and frozen.
It inventories Go packages, imports, structs, interfaces, functions, methods,
constants, and variables with deterministic IDs, exact source locations, and
content digests. Its `1.0.x` line is restricted to compatible defect fixes.

Go Package Identity Inventory **1.0.0** and Go Semantic Inventory **1.0.0** are
released and tagged. The semantic engine deterministically reconciles
declarations, scopes, receivers, types, imports, references, and bounded
interface evidence without network access or repository execution.

Persistence Phases 3.1 and 3.2 are accepted. PostgreSQL benchmark evidence
freezes ordered four-MiB chunks, a four-GiB operational artifact limit, and an
eight-GiB schema ceiling. Phase 3.3 Migration Framework is accepted and frozen
after disposable PostgreSQL 18 validation. Phase 3.4.1 Storage-Neutral
Persistence Port Design, ADR 0013, and Phase 3.4.2 Neutral Go Port and
Conformance Harness are accepted. The Phase 3.4.3 PostgreSQL Adapter and ADR
0014 were accepted on 2026-07-26. Phase 3.4.4 is accepted, and the
storage-neutral Persistence Port and PostgreSQL reference adapter are frozen at
**1.0.0**. Phase 3.5.0 runtime infrastructure design and ADR 0015 are accepted.
Phase 3.5.1 runtime configuration implementation and validation are accepted.
Phase 3.5.2 PostgreSQL runtime implementation and validation are accepted.
Phase 3.5.3 Runtime Lifecycle & Health was accepted on 2026-07-26.
Phase 3.5.4 Runtime Integration & Release Freeze was accepted on 2026-07-27.
Runtime Infrastructure and its five component contracts are frozen at
**1.0.0**. Phase 4.0 Repository Service Layer is authorized. Its design
package and ADR 0016 were accepted on 2026-07-27. The bounded Phase 4.0.1
design spike and Phase 4.0.2 neutral contract/conformance harness are accepted.
Phase 4.0.3 Repository Lifecycle was accepted on 2026-07-27. Phase 4.0.4 Scan
Execution Core was accepted on 2026-07-28. Phase 4.0.5 Intelligence &
Materialization Adapters was accepted on 2026-07-28. Phase 4.0.6 Persistence &
Runtime Integration design was accepted with recommendations on 2026-07-28.
The UUID/manifest golden-vector contracts are frozen and implementation is
authorized. Phase 4.0.7 remains unauthorized.
Intelligence engines remain database-independent.

## Run RIE

```powershell
cd backend
go run ./cmd/rie -path D:\Projects\MyRepository
```

The command emits the versioned JSON schema `1.0.0`.

## Documentation

- [Vision](docs/Vision/PROJECT_VISION.md)
- [Principles](docs/Vision/CORE_PRINCIPLES.md)
- [Version 1 scope](docs/PRD/V1_SCOPE.md)
- [Architecture](docs/Architecture/HIGH_LEVEL_ARCHITECTURE.md)
- [Technology decisions](docs/Architecture/TECHNOLOGY_STACK.md)
- [Module map](docs/Architecture/MODULES.md)
- [Roadmap](docs/Roadmap/DEVELOPMENT_ROADMAP.md)
- [Artifact dependency graph](docs/Architecture/ARTIFACT_DEPENDENCY_GRAPH.md)
- [RIE public API](docs/API/RIE_PUBLIC_API_V1.md)
- [RIE stabilization report](docs/Roadmap/RIE_STABILIZATION_REPORT.md)
- [RIE real-world validation report](docs/Validation/RIE_VALIDATION_REPORT.md)
- [LIE architecture candidate](docs/Architecture/LANGUAGE_INTELLIGENCE_ENGINE.md)
- [Go Language Engine candidate](docs/Architecture/GO_LANGUAGE_ENGINE.md)
- [LIE implementation roadmap](docs/Roadmap/LIE_IMPLEMENTATION_ROADMAP.md)
- [GoLanguageInventory 1.0 public API](docs/API/GO_LANGUAGE_INVENTORY_V1.md)
- [GoLanguageInventory 1.0 release package](docs/Releases/GoLanguageInventory-1.0.0/README.md)
- [PostgreSQL benchmark report](docs/Validation/POSTGRESQL_PAYLOAD_BENCHMARK_REPORT.md)
- [PostgreSQL migration framework](docs/Database/POSTGRESQL_MIGRATION_FRAMEWORK.md)
- [Phase 3.3 migration validation](docs/Validation/POSTGRESQL_MIGRATION_FRAMEWORK_VALIDATION_REPORT.md)
- [Storage-neutral persistence port design](docs/Architecture/STORAGE_NEUTRAL_PERSISTENCE_PORT.md)
- [Persistence Port 1.0 public API](docs/API/PERSISTENCE_PORT_V1.md)
- [Phase 3.4.2 persistence port validation](docs/Validation/PERSISTENCE_PORT_CONFORMANCE_VALIDATION_REPORT.md)
- [Phase 3.4.3 PostgreSQL adapter validation](docs/Validation/POSTGRESQL_ADAPTER_VALIDATION_REPORT.md)
- [Phase 3.4.4 persistence stabilization](docs/Validation/PERSISTENCE_CONTRACT_STABILIZATION_REPORT.md)
- [Persistence Port 1.0 release package](docs/Releases/PersistencePort-1.0.0/README.md)
- [Phase 3.5 runtime architecture](docs/Architecture/RUNTIME_INFRASTRUCTURE.md)
- [Runtime configuration specification](docs/Architecture/RUNTIME_CONFIGURATION_SPECIFICATION.md)
- [Runtime lifecycle specification](docs/Architecture/RUNTIME_LIFECYCLE_SPECIFICATION.md)
- [Health and observability specification](docs/Architecture/HEALTH_OBSERVABILITY_SPECIFICATION.md)
- [ADR 0015 — Runtime infrastructure](docs/Decisions/0015-runtime-infrastructure.md)
- [Runtime infrastructure validation plan](docs/Validation/RUNTIME_INFRASTRUCTURE_VALIDATION_PLAN.md)
- [Phase 3.5.1 runtime configuration validation](docs/Validation/RUNTIME_CONFIGURATION_VALIDATION_REPORT.md)
- [Phase 3.5.2 PostgreSQL runtime validation](docs/Validation/POSTGRESQL_RUNTIME_VALIDATION_REPORT.md)
- [Phase 3.5.3 lifecycle and health validation](docs/Validation/RUNTIME_LIFECYCLE_HEALTH_VALIDATION_REPORT.md)
- [Phase 3.5.4 runtime integration and freeze validation](docs/Validation/RUNTIME_INTEGRATION_FREEZE_VALIDATION_REPORT.md)
- [Runtime 1.0 release-candidate package](docs/Releases/RuntimeInfrastructure-1.0.0/README.md)
- [Runtime deployment runbook](docs/Operations/RUNTIME_DEPLOYMENT_RUNBOOK.md)
- [Runtime operator release checklist](docs/Operations/RUNTIME_RELEASE_CHECKLIST.md)
- [Repository Service Layer architecture](docs/Architecture/REPOSITORY_SERVICE_LAYER.md)
- [Repository Service candidate API](docs/API/REPOSITORY_SERVICE_CANDIDATE_API.md)
- [ADR 0016 — Repository Service Layer](docs/Decisions/0016-repository-service-layer.md)
- [Phase 4 Repository Service roadmap](docs/Roadmap/PHASE_4_REPOSITORY_SERVICE_ROADMAP.md)
- [Repository Service validation plan](docs/Validation/REPOSITORY_SERVICE_LAYER_VALIDATION_PLAN.md)
- [Phase 4.0.1 Repository Service design-spike report](docs/Validation/REPOSITORY_SERVICE_DESIGN_SPIKE_REPORT.md)
- [Phase 4.0.2 Repository Service contract validation](docs/Validation/REPOSITORY_SERVICE_CONTRACT_VALIDATION_REPORT.md)
- [Phase 4.0.3 Repository Lifecycle validation](docs/Validation/REPOSITORY_LIFECYCLE_VALIDATION_REPORT.md)
- [Phase 4.0.4 Scan Execution Core validation](docs/Validation/SCAN_EXECUTION_CORE_VALIDATION_REPORT.md)
- [Phase 4.0.5 Intelligence & Materialization Adapters validation](docs/Validation/INTELLIGENCE_MATERIALIZATION_ADAPTERS_VALIDATION_REPORT.md)
- [Phase 4.0.6 persistence/runtime integration architecture](docs/Architecture/REPOSITORY_SERVICE_PERSISTENCE_RUNTIME_INTEGRATION.md)
- [Phase 4.0.6 integration candidate API](docs/API/REPOSITORY_SERVICE_INTEGRATION_CANDIDATE_API.md)
- [Phase 4.0.6 integration golden vectors](docs/API/REPOSITORY_SERVICE_INTEGRATION_GOLDEN_VECTORS.md)
- [ADR 0017 — Repository Service persistence/runtime integration](docs/Decisions/0017-repository-service-persistence-runtime-integration.md)
- [Phase 4.0.6 integration validation plan](docs/Validation/PERSISTENCE_RUNTIME_INTEGRATION_VALIDATION_PLAN.md)
- [Complete project tests, metrics, and remaining work](PROJECT_TESTS_METRICS_AND_REMAINING_WORK.txt)

## Next milestone

Phases 3.4.1 through 3.4.4 are accepted. Persistence Port and PostgreSQL
Adapter **1.0.0** are frozen. Phases 3.5.1 through 3.5.3 are accepted.
Phase 3.5.4 Runtime Integration & Release Freeze is accepted and Runtime
Infrastructure **1.0.0** is frozen. Phase 4.0 Repository Service Layer is the
only authorized next milestone. The Phase 4.0 design package is accepted, but
production implementation remains gated. Phase 4.0.1 spike evidence is
accepted, and Phase 4.0.2 through Phase 4.0.5 are accepted. Phase 4.0.6 design
is accepted with recommendations and production implementation is authorized
under frozen golden-vector conditions. Phase 4.0.7, HTTP health endpoints,
REST/gRPC, UI, LLM, patch generation, and repository mutation remain
unauthorized.
