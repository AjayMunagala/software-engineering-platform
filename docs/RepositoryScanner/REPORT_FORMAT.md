# Repository Report Format

## Principles

The JSON report is the canonical output. Every inference includes evidence paths and a rule identifier. Counts may be zero; unknown values must be represented as `unknown`, never guessed.

```json
{
  "schemaVersion": "1.0",
  "repository": { "name": "ERP", "rootPath": "D:/Projects/ERP" },
  "summary": { "totalFiles": 3248, "totalDirectories": 412 },
  "languages": [{ "name": "Go", "fileCount": 1420, "evidence": ["go.mod"] }],
  "manifests": [{ "path": "go.mod", "type": "go-module" }],
  "dependencies": [{ "name": "github.com/gin-gonic/gin", "source": "go.mod" }],
  "frameworks": [{ "name": "Gin", "rule": "go-module:gin", "evidence": ["go.mod"] }],
  "tooling": { "build": [], "test": [], "docker": [], "ciCd": [] },
  "layout": { "controllers": 0, "services": 0, "repositories": 0, "sqlTables": 0, "storedProcedures": 0 },
  "configFiles": [],
  "warnings": []
}
```

The layout counts are optional capabilities: they are omitted or marked `unknown` until their extraction rules are implemented and tested.
