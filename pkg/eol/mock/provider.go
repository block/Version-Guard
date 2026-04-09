package mock

import (
	"context"
	"fmt"

	"github.com/block/Version-Guard/pkg/types"
)

// EOLProvider is a mock implementation of eol.Provider for testing
type EOLProvider struct {
	Versions      map[string]*types.VersionLifecycle // key: "engine:version"
	ListErr       error
	GetVersionErr error
}

// GetVersionLifecycle returns mock lifecycle data
func (m *EOLProvider) GetVersionLifecycle(ctx context.Context, engine, version string) (*types.VersionLifecycle, error) {
	if m.GetVersionErr != nil {
		return nil, m.GetVersionErr
	}

	key := fmt.Sprintf("%s:%s", engine, version)
	if lifecycle, ok := m.Versions[key]; ok {
		return lifecycle, nil
	}

	// Return unknown lifecycle if not found
	return &types.VersionLifecycle{
		Version: version,
		Engine:  engine,
		Source:  "mock",
	}, nil
}

// ListAllVersions returns all mock versions for an engine
func (m *EOLProvider) ListAllVersions(ctx context.Context, engine string) ([]*types.VersionLifecycle, error) {
	if m.ListErr != nil {
		return nil, m.ListErr
	}

	var versions []*types.VersionLifecycle
	for _, lifecycle := range m.Versions {
		if lifecycle.Engine == engine {
			versions = append(versions, lifecycle)
		}
	}

	return versions, nil
}

// Name returns the name of this mock provider
func (m *EOLProvider) Name() string {
	return "mock-eol"
}

// Engines returns supported engines
func (m *EOLProvider) Engines() []string {
	return []string{"aurora-mysql", "aurora-postgresql", "postgres", "mysql"}
}
