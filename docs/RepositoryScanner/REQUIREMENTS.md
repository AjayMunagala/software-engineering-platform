# Repository Scanner Requirements

## Mission

Create an accurate, read-only description of a local repository. It provides facts; it does not reason about bugs, edit code, or invoke AI models.

## Inputs

- Absolute repository root path.
- Optional scan settings, such as whether to honor `.gitignore` (default: yes).

## Outputs

- A versioned `RepositoryReport` JSON document.
- A human-readable summary derived from that JSON.
- Warnings for unreadable files, skipped directories, and unsupported formats.

## Detection targets

- Repository identity and filesystem summary.
- Languages and file counts.
- Dependency manifests and declared dependencies.
- Frameworks, configuration files, build/test tools.
- Docker and CI/CD configuration.
- Basic source-layout patterns such as controllers, services, repositories, SQL tables, and stored procedures, only when matching documented rules.

## Explicit constraints

Read-only; local-only; deterministic; no LLM; no shelling out to project tools; no source-content upload; no claim without supporting evidence.
