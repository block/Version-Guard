package wiz

import (
	"context"
	"io"

	"github.com/stretchr/testify/mock"
)

// MockWizClient is a mock implementation of WizClient using testify/mock
type MockWizClient struct {
	mock.Mock
}

// GetAccessToken mocks the Wiz authentication API call
func (m *MockWizClient) GetAccessToken(ctx context.Context) (string, error) {
	args := m.Called(ctx)
	return args.String(0), args.Error(1)
}

// GetReport mocks the Wiz report metadata API call
func (m *MockWizClient) GetReport(ctx context.Context, accessToken, reportID string) (*Report, error) {
	args := m.Called(ctx, accessToken, reportID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	//nolint:errcheck // type assertion in mock is safe
	report, _ := args.Get(0).(*Report)
	return report, args.Error(1)
}

// DownloadReport mocks the Wiz report download (CSV)
func (m *MockWizClient) DownloadReport(ctx context.Context, url string) (io.ReadCloser, error) {
	args := m.Called(ctx, url)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	//nolint:errcheck // type assertion in mock is safe
	reader, _ := args.Get(0).(io.ReadCloser)
	return reader, args.Error(1)
}
