# Disposable PostgreSQL Adapter Validation

`validate.sh` creates an isolated PostgreSQL 18 cluster under `/tmp`, applies
the committed Atlas migrations, runs the neutral conformance suite first, then
runs PostgreSQL-specific rollback and ordered 4 MiB chunk tests. It stops the
cluster and removes the exact temporary directory on exit.

From Windows with Ubuntu WSL:

```text
wsl -d Ubuntu-24.04 -u postgres -- bash /mnt/d/Project_Ai/backend/internal/storage/postgres/tests/validate.sh
```

The script uses trust authentication only inside the disposable loopback-only
cluster. It does not use or store the installed Windows/Ubuntu database
credentials.

