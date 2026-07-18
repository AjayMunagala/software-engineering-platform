# Version 1 Scope — Local Repository Intelligence

## Goal

Produce a reliable factual inventory of a local software repository. This is the foundation for every later reasoning feature.

## In scope

- Scan a user-selected local repository.
- Detect repository name and root path.
- List files and folders while respecting ignore rules.
- Detect languages from file extensions and known manifests.
- Find dependency manifests and extract declared dependencies where supported.
- Identify configuration files, build tools, test tools, and likely frameworks.
- Produce a machine-readable report and a readable summary.
- Support an initial small language set: Go, TypeScript/JavaScript, React projects, SQL files, JSON, YAML, and Markdown.

## Out of scope

- LLM reasoning or chat.
- Editing code or generating patches.
- Running builds, tests, migrations, or arbitrary commands.
- Reading production logs, databases, credentials, or remote systems.
- Voice, image generation, cloud collaboration, and autonomous actions.

## Acceptance criteria

Given a valid local repository, the scanner returns a deterministic report containing repository metadata, detected languages, directory summary, file counts, manifests, dependencies, configuration files, frameworks, and build/test tooling. It must clearly report unsupported formats and never invent missing data.
