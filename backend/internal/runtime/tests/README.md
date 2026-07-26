# Runtime integration validation

`validate_linux.sh` runs the runtime package regression, three shuffled runs,
`go vet`, and race detection on Linux with the module's pinned Go 1.26.2
toolchain. It creates no database, network listener, credential, or product
data.

Example from Ubuntu/WSL when Go 1.26.2 is installed outside the repository:

```bash
AEGIS_GO_BINARY=/tmp/aegis-go/bin/go bash validate_linux.sh
```

For a disposable, SHA-256-verified official toolchain that is removed after
the run:

```bash
bash bootstrap_linux.sh
```

The PostgreSQL-backed disposable validation remains in
`../postgres/tests/validate.sh`.
