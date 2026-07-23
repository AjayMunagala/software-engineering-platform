# ADR 0004: PostgreSQL for Structured Persistence

## Status

Superseded on 2026-07-23 by accepted ADR 0010, which defines the artifact
ownership, exact-payload, transaction, and adapter boundaries more precisely.

## Decision

Use PostgreSQL as the future durable relational store.

## Rationale

It supports structured repository metadata, auditability, transactions, and mature operational tooling. It is not a Version 1 dependency.
