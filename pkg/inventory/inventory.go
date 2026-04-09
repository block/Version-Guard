package inventory

import (
	"context"

	"github.com/block/Version-Guard/pkg/types"
)

// InventorySource defines the interface for fetching cloud resource inventory
//
//nolint:revive // InventorySource is intentionally verbose for clarity
type InventorySource interface {
	// ListResources returns all resources of the specified type
	ListResources(ctx context.Context, resourceType types.ResourceType) ([]*types.Resource, error)

	// GetResource returns a specific resource by ID
	GetResource(ctx context.Context, resourceType types.ResourceType, id string) (*types.Resource, error)

	// Name returns the name of this inventory source (e.g., "wiz", "aws-api")
	Name() string

	// CloudProvider returns which cloud provider this source supports
	CloudProvider() types.CloudProvider
}
