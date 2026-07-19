# RIE 1.0.0 Public API

## Stability promise

The APIs and artifact contracts listed here are frozen for RIE 1.x. Additive compatible changes require a minor version. Breaking contract changes require RIE 2.0.0 or a new artifact major version.

## Pipeline API

- `rie.New()` creates an empty ordered pipeline.
- `Pipeline.Register(Engine)` registers one uniquely named engine.
- `Pipeline.Run(context.Context, *RunContext)` executes engines in registration order.
- `rie.DefaultConfig()` provides conservative local scan defaults.
- `rie.NewRunContext(path, config)` initializes one scan.
- `rie.Version` and `rie.SchemaVersion` are both `1.0.0`.

`RunContext.Entries` is an internal Discovery-to-Ignore compatibility bridge. It is public only because engines live in separate Go packages. New engines must not consume it.

## Engine contract

Every engine implements:

```go
type Engine interface {
    Name() string
    Version() string
    Description() string
    Execute(context.Context, *RunContext) error
}
```

Concrete engine implementations are private. Consumers construct engines through each package's `New` function.

## Artifact API

- `Artifact` exposes stable name and version.
- `ArtifactStore.Put` publishes once; duplicate names fail.
- `ArtifactStore.Get` retrieves by name.
- `ArtifactAs[T]` performs typed retrieval.
- Every engine package exposes `InventoryFrom(run)` for its stable artifact.

| Artifact | Version | Producer |
|---|---:|---|
| `DiscoveryInventory` | 1.0.0 | Discovery |
| `RepositorySnapshot` | 1.0.0 | Ignore |
| `LanguageInventory` | 1.0.0 | Language |
| `FrameworkInventory` | 1.0.0 | Framework |
| `BuildInventory` | 1.0.0 | Build & Package Intelligence |
| `RepositoryMetadata` | 1.0.0 | Repository Metadata |
| `RepositoryIntelligenceSummary` | 1.0.0 | Repository Intelligence Summary |

Artifacts are immutable: internal slices are private, nested collections are defensively copied, and publication is single-assignment.

## Extension API

Build & Package Intelligence intentionally exposes its Go `Detector` registry contract through `build.Config.Detectors`. It is the only RIE 1.0 runtime detector extension point. Framework rules are not yet a public registry.

## JSON contract

`rie.Report` is the additive JSON schema `1.0.0`. Engines never consume report fields. Unknown intelligence remains explicitly unavailable instead of being represented as zero or fabricated data.

## Compatibility policy

- Existing JSON fields are not removed or retyped in 1.x.
- Existing artifact accessors keep their semantics in 1.x.
- New optional fields may be added with a schema minor version.
- Detection coverage may grow without an API version change when existing meanings remain unchanged.
- Engine implementation versions may receive patches without changing artifact major versions.
