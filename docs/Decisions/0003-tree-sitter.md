# ADR 0003: Tree-sitter for Syntax Parsing

## Status

Proposed.

## Decision

Use Tree-sitter as the common multi-language syntax parsing layer.

## Rationale

It provides incremental parsing and broad language support. Language-native parsers may supplement it where deeper semantic analysis is necessary.
