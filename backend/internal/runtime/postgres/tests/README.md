# Disposable PostgreSQL runtime validation

`validate.sh` creates a fresh PostgreSQL 18 cluster under `/tmp`, applies the
accepted checksum-verified migrations, creates temporary combined and
capability-specific logins, generates a disposable certificate authority, and
runs disabled-TLS, verify-full-TLS, and application lifecycle tests from
Windows. The cluster,
database, roles, certificates, and data are destroyed when the script exits.

Run from PowerShell:

```powershell
wsl.exe -d Ubuntu-24.04 -u postgres -- bash -lc `
  'cd /mnt/d/Project_Ai/backend/internal/runtime/postgres/tests && bash validate.sh'
```

The harness uses no persistent database, personal credentials, committed
passwords, APIs, health subsystem, or application startup coordinator.
