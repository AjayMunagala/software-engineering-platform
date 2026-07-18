# Functional Requirements

## Repository Scanner (Version 1)

| ID | Requirement |
| --- | --- |
| FR-01 | Accept a local repository root path. |
| FR-02 | Validate that the path exists, is a directory, and can be read. |
| FR-03 | Traverse files while honoring `.gitignore` and standard excluded directories. |
| FR-04 | Count files and directories without reading binary file contents. |
| FR-05 | Detect languages from extensions and supported manifests. |
| FR-06 | Detect manifests, dependencies, configuration files, build tools, test tools, Docker, and CI/CD definitions. |
| FR-07 | Identify supported frameworks using documented rules and source references. |
| FR-08 | Emit JSON matching the report format and a readable summary. |
| FR-09 | Record unreadable paths, unsupported formats, and skipped files as warnings. |
| FR-10 | Never alter repository files or run project commands. |
