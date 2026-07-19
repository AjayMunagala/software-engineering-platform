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

Only manifest files retained by Ignore Engine are inspected. Reads are bounded by `MaxManifestSize`. Every detection records the relative manifest path that supports it. Invalid, unreadable, or oversized manifests produce standardized warnings and do not abort the scan.

Future engines consume `FrameworkInventory`; the JSON framework section is presentation output.

## Package standard

- `interface.go` — public engine contract
- `implementation.go` — bounded manifest detector
- `config.go` — manifest names and size limit
- `model.go` — immutable inventory artifact
- `errors.go` — stable sentinel errors
- `implementation_test.go` — behavior tests
- `implementation_benchmark_test.go` — performance benchmark
