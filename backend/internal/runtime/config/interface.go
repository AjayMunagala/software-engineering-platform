// Package config loads, validates, and freezes application runtime
// configuration. It owns no database pool, network listener, lifecycle,
// health, observability, migration, persistence, or engine behavior.
package config

import "context"

// ContractVersion identifies the candidate Phase 3.5.1 configuration API.
const ContractVersion = "0.1.0"

// Loader resolves ordinary configuration sources and selects exactly one
// secret provider without creating any runtime resource.
type Loader interface {
	Load(context.Context, LoadRequest) (LoadedConfiguration, error)
}

// SecretProvider resolves a named secret for a later, authorized runtime
// consumer. Implementations must return a detached byte slice.
type SecretProvider interface {
	Resolve(context.Context, SecretReference) ([]byte, error)
}

// LoadRequestParams is mutable caller input used only by NewLoadRequest.
// Arguments exclude the process executable name.
type LoadRequestParams struct {
	Environment    []string
	Arguments      []string
	SecretProvider SecretProvider
}

// LoadRequest is detached from caller-owned slices.
type LoadRequest struct {
	environment    []string
	arguments      []string
	secretProvider SecretProvider
}

// NewLoadRequest constructs an immutable load request.
func NewLoadRequest(params LoadRequestParams) LoadRequest {
	return LoadRequest{
		environment:    append([]string(nil), params.Environment...),
		arguments:      append([]string(nil), params.Arguments...),
		secretProvider: params.SecretProvider,
	}
}

// Environment returns a detached copy for diagnostics and testing.
func (request LoadRequest) Environment() []string {
	return append([]string(nil), request.environment...)
}

// Arguments returns a detached copy for diagnostics and testing.
func (request LoadRequest) Arguments() []string {
	return append([]string(nil), request.arguments...)
}
