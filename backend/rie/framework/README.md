# RIE v0.4 — Framework Engine

Framework Engine consumes the frozen `LanguageInventory` artifact and detects frameworks deterministically from declared dependencies in supported manifest files. It does not execute package managers, parse source code, invoke AST tooling, or use AI.

## Supported manifests

- `go.mod`: Gin, Echo, Fiber, Chi
- `package.json`: React, Redux, Next.js, Vue, Angular, Express, NestJS, Svelte
- `pom.xml`: Spring Boot, Spring Framework
- `Cargo.toml`: Actix Web, Axum, Rocket
- `composer.json`: Laravel, Symfony
- `requirements.txt`: Django, Flask, FastAPI

## Safety and evidence

Only manifest files retained by Ignore Engine are inspected. Reads are bounded by `MaxManifestSize`. Every detection records shared `rie.Evidence` containing the repository-relative manifest file, the deterministic rule, and the matched dependency value. The manifest's parent directory identifies the project location in multi-project repositories. Invalid, unreadable, or oversized manifests produce standardized warnings and do not abort the scan.

## Detection policy

- Frameworks are reported independently. For example, a manifest declaring React, Redux, and Next.js reports all three; framework implications are not used to remove detections.
- Frameworks may coexist. React and Vue in the same repository, or even the same manifest, are valid independent detections and do not produce conflict warnings.
- The same framework found in multiple projects is represented once with separate location-aware evidence for every supporting manifest.
- Exact duplicate evidence is removed. Distinct dependency values supporting the same framework remain visible.
- The engine is read-only and never executes package managers or build tools.

Future engines consume `FrameworkInventory`; the JSON framework section is presentation output.

## Frozen API

`FrameworkInventory` artifact version `1.0.0` is the stable contract for later engines. Its state is immutable to consumers and all returned evidence is defensively copied.

## Planned technical debt

Framework definitions currently remain close to their manifest parsers to keep v0.4 focused. Move definitions to a registry such as `frameworks.json` once the number of supported frameworks or manifest types justifies the additional infrastructure.

## Package standard

- `interface.go` — public engine contract
- `implementation.go` — bounded manifest detector
- `config.go` — manifest names and size limit
- `model.go` — immutable inventory artifact
- `errors.go` — stable sentinel errors
- `implementation_test.go` — behavior tests
- `implementation_benchmark_test.go` — performance benchmark
