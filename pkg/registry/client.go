package registry

import (
	"context"
	"errors"
)

var (
	// ErrNotFound is returned when a service is not found in the registry
	ErrNotFound = errors.New("service not found in registry")
)

// Client queries your service registry for service metadata.
// The registry maps cloud accounts to owning services and teams.
type Client interface {
	// GetServiceByAWSAccount looks up service metadata by AWS account ID and region.
	// Returns ErrNotFound if the account is not registered.
	GetServiceByAWSAccount(ctx context.Context, accountID, region string) (*ServiceInfo, error)
}

// ServiceInfo contains metadata about a service from the registry.
type ServiceInfo struct {
	// ServiceName is the canonical service name (e.g., "payments")
	ServiceName string

	// Team is the owning team name
	Team string

	// Environment is the deployment environment (production, staging, development)
	Environment string
}
