package mock

import (
	"context"
	"fmt"

	"github.com/block/Version-Guard/pkg/types"
)

// InventorySource is a mock implementation of inventory.InventorySource for testing
//
//nolint:govet // field alignment sacrificed for readability
type InventorySource struct {
	Resources      []*types.Resource
	GetResourceErr error
	ListErr        error
}

// ListResources returns the mock resources
func (m *InventorySource) ListResources(ctx context.Context, resourceType types.ResourceType) ([]*types.Resource, error) {
	if m.ListErr != nil {
		return nil, m.ListErr
	}

	var filtered []*types.Resource
	for _, r := range m.Resources {
		if r.Type == resourceType {
			filtered = append(filtered, r)
		}
	}

	return filtered, nil
}

// GetResource returns a specific mock resource
func (m *InventorySource) GetResource(ctx context.Context, resourceType types.ResourceType, id string) (*types.Resource, error) {
	if m.GetResourceErr != nil {
		return nil, m.GetResourceErr
	}

	for _, r := range m.Resources {
		if r.ID == id && r.Type == resourceType {
			return r, nil
		}
	}

	return nil, fmt.Errorf("resource not found: %s", id)
}

// Name returns the name of this mock source
func (m *InventorySource) Name() string {
	return "mock-inventory"
}

// CloudProvider returns the cloud provider
func (m *InventorySource) CloudProvider() types.CloudProvider {
	return types.CloudProviderAWS
}
