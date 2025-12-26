package server

import "context"

// Server represents a remote server that can be configured.
type Server interface {
	// ID returns a unique identifier for the server.
	ID() string
	// Address returns the connection address (IP or hostname).
	Address() string
	// Execute runs a command on the server.
	Execute(ctx context.Context, command string) (string, error)
}

// Configurator defines the interface for applying configurations to a server.
type Configurator interface {
	// Configure applies the given configuration steps to the server.
	Configure(ctx context.Context, s Server) error
}
