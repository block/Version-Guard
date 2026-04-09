package aws

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRDSEOLProvider_GetVersionLifecycle_DeprecatedVersion(t *testing.T) {
	ctx := context.Background()

	// Setup: Create mock RDS client with AWS API response
	mockClient := new(MockRDSClient)
	mockClient.On("DescribeDBEngineVersions", mock.Anything, "aurora-mysql").
		Return([]*EngineVersion{AWSAPIFixtures.DeprecatedMySQLVersion}, nil)

	provider := NewRDSEOLProvider(mockClient, time.Hour)

	// Execute: Get lifecycle for deprecated version
	lifecycle, err := provider.GetVersionLifecycle(ctx, "aurora-mysql", "5.6.10a")

	// Verify: No error
	require.NoError(t, err)
	require.NotNil(t, lifecycle)

	// Verify: Lifecycle correctly parsed from AWS API response
	assert.Equal(t, "5.6.10a", lifecycle.Version)
	assert.Equal(t, "aurora-mysql", lifecycle.Engine)
	assert.True(t, lifecycle.IsDeprecated, "Should be marked as deprecated")
	assert.False(t, lifecycle.IsSupported, "Should not be supported")
	assert.NotNil(t, lifecycle.DeprecationDate)
	assert.Equal(t, "aws-rds-api", lifecycle.Source)

	// Verify: Mock was called correctly
	mockClient.AssertExpectations(t)
}

func TestRDSEOLProvider_GetVersionLifecycle_CurrentVersion(t *testing.T) {
	ctx := context.Background()

	mockClient := new(MockRDSClient)
	mockClient.On("DescribeDBEngineVersions", mock.Anything, "aurora-mysql").
		Return([]*EngineVersion{AWSAPIFixtures.CurrentMySQLVersion}, nil)

	provider := NewRDSEOLProvider(mockClient, time.Hour)

	lifecycle, err := provider.GetVersionLifecycle(ctx, "aurora-mysql", "8.0.mysql_aurora.3.05.2")

	require.NoError(t, err)
	require.NotNil(t, lifecycle)

	// Verify: Current version is supported
	assert.True(t, lifecycle.IsSupported, "Current version should be supported")
	assert.False(t, lifecycle.IsDeprecated, "Current version should not be deprecated")
	assert.False(t, lifecycle.IsEOL, "Current version should not be EOL")
	assert.False(t, lifecycle.IsExtendedSupport, "Current version should not be in extended support")

	mockClient.AssertExpectations(t)
}

func TestRDSEOLProvider_GetVersionLifecycle_ExtendedSupport(t *testing.T) {
	ctx := context.Background()

	mockClient := new(MockRDSClient)
	mockClient.On("DescribeDBEngineVersions", mock.Anything, "aurora-mysql").
		Return([]*EngineVersion{AWSAPIFixtures.ExtendedSupportVersion}, nil)

	provider := NewRDSEOLProvider(mockClient, time.Hour)

	lifecycle, err := provider.GetVersionLifecycle(ctx, "aurora-mysql", "5.7.12")

	require.NoError(t, err)
	require.NotNil(t, lifecycle)

	// Verify: Extended support detection
	// DeprecationDate in past + Status "available" = extended support
	assert.True(t, lifecycle.IsExtendedSupport, "Should be in extended support")
	assert.True(t, lifecycle.IsSupported, "Extended support is still supported")
	assert.NotNil(t, lifecycle.ExtendedSupportEnd, "Should have extended support end date")

	mockClient.AssertExpectations(t)
}

func TestRDSEOLProvider_GetVersionLifecycle_EOLNoUpgradePath(t *testing.T) {
	ctx := context.Background()

	mockClient := new(MockRDSClient)
	mockClient.On("DescribeDBEngineVersions", mock.Anything, "aurora-mysql").
		Return([]*EngineVersion{AWSAPIFixtures.VersionWithNoUpgradePath}, nil)

	provider := NewRDSEOLProvider(mockClient, time.Hour)

	lifecycle, err := provider.GetVersionLifecycle(ctx, "aurora-mysql", "5.6.10a")

	require.NoError(t, err)
	require.NotNil(t, lifecycle)

	// Verify: Deprecated + No upgrade path = EOL
	assert.True(t, lifecycle.IsEOL, "Deprecated with no upgrade path should be EOL")
	assert.True(t, lifecycle.IsDeprecated)
	assert.False(t, lifecycle.IsSupported)
	assert.NotNil(t, lifecycle.EOLDate)

	mockClient.AssertExpectations(t)
}

func TestRDSEOLProvider_GetVersionLifecycle_VersionNotFound(t *testing.T) {
	ctx := context.Background()

	mockClient := new(MockRDSClient)
	mockClient.On("DescribeDBEngineVersions", mock.Anything, "aurora-mysql").
		Return([]*EngineVersion{AWSAPIFixtures.CurrentMySQLVersion}, nil)

	provider := NewRDSEOLProvider(mockClient, time.Hour)

	// Request a version that doesn't exist in the AWS response
	lifecycle, err := provider.GetVersionLifecycle(ctx, "aurora-mysql", "99.99.99")

	require.NoError(t, err)
	require.NotNil(t, lifecycle)

	// Verify: Returns unknown lifecycle with empty Version (signals missing data, not unsupported version)
	assert.Equal(t, "", lifecycle.Version, "Version not found should return empty Version to signal data gap")
	assert.Equal(t, "aurora-mysql", lifecycle.Engine)
	assert.False(t, lifecycle.IsSupported, "Unknown version should not be supported")

	mockClient.AssertExpectations(t)
}

func TestRDSEOLProvider_ListAllVersions(t *testing.T) {
	ctx := context.Background()

	mockClient := new(MockRDSClient)
	mockClient.On("DescribeDBEngineVersions", mock.Anything, "aurora-mysql").
		Return(AWSAPIFixtures.AuroraMySQLVersions, nil)

	provider := NewRDSEOLProvider(mockClient, time.Hour)

	versions, err := provider.ListAllVersions(ctx, "aurora-mysql")

	require.NoError(t, err)
	require.Len(t, versions, 5, "Should return all Aurora MySQL versions")

	// Verify: All versions are converted correctly
	for _, v := range versions {
		assert.Equal(t, "aurora-mysql", v.Engine)
		assert.NotEmpty(t, v.Version)
		assert.Equal(t, "aws-rds-api", v.Source)
	}

	mockClient.AssertExpectations(t)
}

func TestRDSEOLProvider_ListAllVersions_Caching(t *testing.T) {
	ctx := context.Background()

	mockClient := new(MockRDSClient)
	// Mock should only be called ONCE due to caching
	mockClient.On("DescribeDBEngineVersions", mock.Anything, "aurora-mysql").
		Return(AWSAPIFixtures.AuroraMySQLVersions, nil).
		Once()

	provider := NewRDSEOLProvider(mockClient, time.Hour)

	// First call - should hit AWS API
	versions1, err1 := provider.ListAllVersions(ctx, "aurora-mysql")
	require.NoError(t, err1)
	require.Len(t, versions1, 5)

	// Second call - should use cache (mock not called again)
	versions2, err2 := provider.ListAllVersions(ctx, "aurora-mysql")
	require.NoError(t, err2)
	require.Len(t, versions2, 5)

	// Verify: Same data returned
	assert.Equal(t, versions1, versions2)

	// Verify: Mock was called exactly once
	mockClient.AssertExpectations(t)
}

func TestRDSEOLProvider_Name(t *testing.T) {
	mockClient := new(MockRDSClient)
	provider := NewRDSEOLProvider(mockClient, time.Hour)

	assert.Equal(t, "aws-rds-api", provider.Name())
}

func TestRDSEOLProvider_Engines(t *testing.T) {
	mockClient := new(MockRDSClient)
	provider := NewRDSEOLProvider(mockClient, time.Hour)

	engines := provider.Engines()
	assert.Contains(t, engines, "aurora-mysql")
	assert.Contains(t, engines, "aurora-postgresql")
	assert.Contains(t, engines, "mysql")
	assert.Contains(t, engines, "postgres")
}
