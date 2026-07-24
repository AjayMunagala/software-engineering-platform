# Phase 3.3 Migration Validation

This suite validates the migration framework against a PostgreSQL 18 cluster
that it initializes under a temporary directory. It creates uniquely named
databases, runs the checks, stops the isolated server, and removes its data
directory on exit. It does not use application credentials, store artifacts,
or connect to the installed default PostgreSQL cluster.

Prerequisites:

- PostgreSQL 18 server and client tools, including `initdb` and `pg_ctl`;
- Atlas Community CLI v1.2.3;
- execution as the local `postgres` operating-system user;
- repository available on the local filesystem.

Example from Windows with the project's Ubuntu WSL distribution:

```text
wsl -d Ubuntu-24.04 -u postgres -- \
  bash /mnt/d/Project_Ai/backend/persistence/postgres/tests/validate.sh
```

The suite covers checksum tampering, empty install, partial upgrade, repeat
apply, newer-database rejection, transactional rollback, concurrent apply,
schema ownership, capability roles, runtime DDL denial, and restricted
payload/audit access. All database names include the process identifier and are
removed by a shell trap.
