# ADR 0005: Model-Agnostic Architecture

## Status

Accepted.

## Decision

Keep model-specific behavior behind a stable adapter boundary.

## Rationale

Models will improve and change. Repository intelligence, evidence, validation, and safety workflows must remain durable independent of the selected model.
