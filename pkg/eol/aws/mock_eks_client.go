package aws

import (
	"context"

	"github.com/stretchr/testify/mock"
)

// MockEKSClient is a mock implementation of EKSClient using testify/mock
type MockEKSClient struct {
	mock.Mock
}

// DescribeAddonVersions mocks the AWS EKS DescribeAddonVersions API call
func (m *MockEKSClient) DescribeAddonVersions(ctx context.Context) ([]*EKSVersion, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	//nolint:errcheck // type assertion in mock is safe
	versions, _ := args.Get(0).([]*EKSVersion)
	return versions, args.Error(1)
}
