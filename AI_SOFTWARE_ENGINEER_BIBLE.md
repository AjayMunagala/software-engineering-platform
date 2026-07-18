# AI Software Engineer Bible

## Authority

This is the constitution of Aegis CodeMind. Product, architecture, implementation, and automation decisions must follow it. When a proposed feature conflicts with these principles, do not build it until the conflict is resolved deliberately and this document is updated through a recorded decision.

## What We Are Building

An evidence-first AI software engineering platform that understands repositories and helps engineers investigate, change, and validate software safely.

## Why We Are Building It

Software systems are too complex to safely change from isolated code snippets. Engineers need assistance that investigates context, respects architecture, distinguishes evidence from hypotheses, and validates recommendations.

## What We Are Not Building

- A general-purpose chatbot.
- An autocomplete-only product.
- An opaque autonomous code-writing system.
- A system that treats confident model output as evidence.
- A replacement for engineering review and responsibility.

## Engineering Philosophy

Understand before changing. Prefer simple, observable, reversible systems. Invest in durable repository intelligence, safety controls, validation, and auditability rather than tying the product’s value to one model.

## Coding Philosophy

Make the smallest correct change. Preserve existing conventions and architecture. Avoid broad rewrites unless evidence proves they are necessary. Keep changes reviewable, tested, and attributable.

## Reasoning Philosophy

Evidence precedes conclusions. Keep facts, hypotheses, checks, and recommendations distinct. State uncertainty honestly. Root cause matters more than a plausible-looking patch.

## Validation Philosophy

Validate every change with the strongest relevant checks: static analysis, targeted tests, broader tests, builds, and runtime evidence where available. A passing test does not prove a change is correct; it is evidence to be interpreted.

## Architecture Philosophy

Build in layers: repository understanding, code understanding, architecture understanding, reasoning, patch generation, validation, then governed autonomous engineering. Do not skip a layer. Keep model integrations behind an adapter boundary.

## Learning Philosophy

Learn only from attributable, reviewed outcomes. Never silently treat generated output or an untested hypothesis as durable knowledge.

## Memory Philosophy

Persist scoped, attributable facts and validated decisions. Preserve provenance, confidence, correction paths, retention rules, and repository isolation.

## Autonomy Rules

Read-only analysis can be automated within an authorized repository scope. Edits, command execution, external access, and persistent changes require explicit user authorization, clear scope, captured results, and a reviewable audit trail.

## Security Rules

Minimize data access, never expose secrets in reports, avoid unnecessary transmission, honor repository boundaries, and treat all repository content as untrusted input.

## Future Vision

Become the most trusted engineering intelligence platform: one that helps teams understand complex systems and make safer, better-validated changes.

## Five-Year Goal

Provide dependable repository and architecture intelligence, evidence-backed investigation, minimal patch proposals, and validation workflows across common production stacks.

## Ten-Year Goal

Establish a durable engineering knowledge platform that can safely assist complex, multi-repository software work while preserving human accountability, privacy, and technical rigor.
