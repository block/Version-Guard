package aws

import (
	"context"

	"github.com/stretchr/testify/mock"
)

// MockRDSClient is a mock implementation of RDSClient using testify/mock
type MockRDSClient struct {
	mock.Mock
}

// DescribeDBEngineVersions mocks the AWS RDS DescribeDBEngineVersions API call
func (m *MockRDSClient) DescribeDBEngineVersions(ctx context.Context, engine string) ([]*EngineVersion, error) {
	args := m.Called(ctx, engine)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	//nolint:errcheck // type assertion in mock is safe
	versions, _ := args.Get(0).([]*EngineVersion)
	return versions, args.Error(1)
}
