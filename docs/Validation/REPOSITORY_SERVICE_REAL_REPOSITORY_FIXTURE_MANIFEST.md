# Repository Service Real-Repository Fixture Manifest

## Status

- Phase: 4.0.7 design
- Status: Approved with recommendations on 2026-07-29
- Date: 2026-07-29

## Normative corpus

| ID | Category | Repository | Exact revision | Expected profile branch |
|---|---|---|---|---|
| `non-go-small` | Small non-Go | `sindresorhus/p-limit` | `42599ebbbb1228a5bdab381fcf8f4ac20eb8d551` | seven RIE artifacts |
| `go-cli` | Small Go CLI | `junegunn/fzf` | `235a726fae89bec3ac6d3e7facd2716d78bb625d` | ten Go-profile artifacts |
| `go-service` | Medium Go service | `pocketbase/pocketbase` | `cc4e85709074c8a81284c3d9c5064d2adbf4c854` | ten Go-profile artifacts |
| `go-generics` | Generics-heavy Go library | `samber/lo` | `bb6e45518419e49b59a20a55de900df7698ad644` | ten Go-profile artifacts |
| `go-workspace` | Workspace / multi-module | `kubernetes-sigs/kustomize` | `f7d33c2610d868318bdd4442b32a8616e3b66eb8` | ten Go-profile artifacts |
| `go-external` | Large / unavailable dependencies | `open-telemetry/opentelemetry-go` | `d98b5f42f2550ba2b851d9d95f4ddb6c4f98c4b0` | ten Go-profile artifacts with explicit partial facts |
| `go-kubernetes` | Kubernetes-scale | `kubernetes/kubernetes` | `1056cbb4511a20ae5e4d0173813cd3403c6d836a` | ten Go-profile artifacts with bounded omissions |
| `broken-go` | Malformed source | controlled fixture derived from the accepted Phase 2.2.8 case | frozen by fixture-tree digest before execution | explicit failed/partial evidence |
| `stale-go` | Digest-stale source | controlled fixture derived from the accepted Phase 2.2.8 case | frozen by fixture-tree digest before execution | explicit stale evidence |

The `p-limit` full revision was resolved from the already validated abbreviated
RIE revision. All Go revisions are reused unchanged from the accepted Phase
2.2.8 Go semantic validation corpus.

## Optional coverage case

A framework-rich non-Go repository may be added only through design review.
It cannot replace `non-go-small`, and its full 40-character revision and source
manifest must be frozen before execution. Optional results are reported
separately from mandatory release gates.

## Preflight proof

Before any case runs, the operator records:

- repository URL;
- full 40-character commit;
- Git tree ID;
- clean-status proof;
- tracked file count;
- canonical path/file-kind/size/SHA-256 manifest digest;
- submodule declarations and whether they remain intentionally uninitialized;
- symlink representation;
- checkout tool and exact Git version.

The harness rejects a revision, tree, or canonical-manifest mismatch. It does
not repair, clean, generate, install, or mutate a checkout automatically.

## Source preparation

- Fetching is an operator preflight step, never a Repository Service feature.
- Checkouts live outside the project repository and disposable PostgreSQL data.
- Repositories are detached at the exact revision.
- Dependency install, module download, Git hooks, generators, builds, and tests
  are forbidden.
- The measured service run begins only after network access is removed or
  monitored as zero.
- Native Windows and Ubuntu views must have the same canonical source manifest
  before their exact-byte outputs may be compared.

## Frozen artifact expectations

Every successful repository produces, in order:

1. `discovery-inventory 1.0.0`;
2. `repository-snapshot 1.0.0`;
3. `language-inventory 1.0.0`;
4. `framework-inventory 1.0.0`;
5. `build-inventory 1.0.0`;
6. `repository-metadata 1.0.0`;
7. `repository-intelligence-summary 1.0.0`.

Repositories with recognized Go files additionally produce:

8. `go-language-inventory 1.0.0`;
9. `go-package-identity-inventory 1.0.0`;
10. `go-semantic-inventory 1.0.0`.

Missing, extra, reordered, wrong-version, wrong-codec, or dependency-invalid
artifacts fail the case. Explicit partial semantic states caused by unavailable
external evidence remain valid when they match the released Go LIE contract.

## Controlled-fixture rules

The malformed and stale fixtures are copied from accepted test definitions
into a disposable validation root. Before execution, the design-approved
harness records their canonical tree digests. The stale case permits only the
single documented mutation between syntax publication and semantic source
verification. All other mutation is a validation failure.

## Change control

The corpus is immutable for one Phase 4.0.7 report. Replacing a repository,
changing a revision, initializing a submodule, or changing a controlled fixture
requires a documented design amendment before new evidence is collected.
