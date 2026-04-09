package aws

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/block/Version-Guard/pkg/types"
)

func TestEKSEOLProvider_GetVersionLifecycle(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	standardEnd := now.AddDate(0, 6, 0) // 6 months from now
	extendedEnd := now.AddDate(1, 2, 0) // 14 months from now

	mockClient := new(MockEKSClient)
	provider := NewEKSEOLProvider(mockClient, time.Hour)

	// Mock AWS API response
	mockVersions := []*EKSVersion{
		{
			KubernetesVersion:     "1.29",
			Status:                "standard",
			EndOfStandardDate:     &standardEnd,
			EndOfExtendedDate:     &extendedEnd,
			LatestPlatformVersion: "eks.5",
		},
		{
			KubernetesVersion:     "1.28",
			Status:                "extended",
			EndOfStandardDate:     &now,
			EndOfExtendedDate:     &extendedEnd,
			LatestPlatformVersion: "eks.4",
		},
	}

	mockClient.On("DescribeAddonVersions", ctx).Return(mockVersions, nil)

	tests := []struct {
		name        string
		engine      string
		version     string
		wantVersion string
		wantStatus  func(*types.VersionLifecycle) bool
	}{
		{
			name:        "standard support version",
			engine:      "kubernetes",
			version:     "1.29",
			wantVersion: "k8s-1.29",
			wantStatus: func(v *types.VersionLifecycle) bool {
				return v.IsSupported && !v.IsDeprecated && !v.IsEOL
			},
		},
		{
			name:        "extended support version",
			engine:      "kubernetes",
			version:     "1.28",
			wantVersion: "k8s-1.28",
			wantStatus: func(v *types.VersionLifecycle) bool {
				return v.IsSupported && v.IsExtendedSupport && v.IsDeprecated
			},
		},
		{
			name:        "version with k8s prefix",
			engine:      "kubernetes",
			version:     "k8s-1.29",
			wantVersion: "k8s-1.29",
			wantStatus: func(v *types.VersionLifecycle) bool {
				return v.IsSupported
			},
		},
		{
			name:        "eks engine variant",
			engine:      "eks",
			version:     "1.29",
			wantVersion: "k8s-1.29",
			wantStatus: func(v *types.VersionLifecycle) bool {
				return v.IsSupported
			},
		},
		{
			name:        "unknown version",
			engine:      "kubernetes",
			version:     "1.27",
			wantVersion: "", // Empty Version signals missing data (data gap, not unsupported)
			wantStatus: func(v *types.VersionLifecycle) bool {
				return !v.IsSupported
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lifecycle, err := provider.GetVersionLifecycle(ctx, tt.engine, tt.version)
			require.NoError(t, err)
			require.NotNil(t, lifecycle)

			assert.Equal(t, tt.wantVersion, lifecycle.Version)
			assert.Equal(t, "kubernetes", lifecycle.Engine)
			assert.Equal(t, "aws-eks-api", lifecycle.Source)
			assert.True(t, tt.wantStatus(lifecycle), "status check failed for %s", tt.name)
		})
	}

	mockClient.AssertExpectations(t)
}

func TestEKSEOLProvider_ListAllVersions(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	past := now.AddDate(0, -6, 0)
	future := now.AddDate(0, 6, 0)

	mockClient := new(MockEKSClient)
	provider := NewEKSEOLProvider(mockClient, time.Hour)

	mockVersions := []*EKSVersion{
		{
			KubernetesVersion: "1.29",
			Status:            "standard",
			EndOfStandardDate: &future,
		},
		{
			KubernetesVersion: "1.28",
			Status:            "extended",
			EndOfStandardDate: &past,
			EndOfExtendedDate: &future,
		},
		{
			KubernetesVersion: "1.27",
			Status:            "deprecated",
			EndOfStandardDate: &past,
			EndOfExtendedDate: &past,
		},
	}

	mockClient.On("DescribeAddonVersions", ctx).Return(mockVersions, nil)

	versions, err := provider.ListAllVersions(ctx, "kubernetes")
	require.NoError(t, err)
	require.Len(t, versions, 3)

	// Check standard support version
	assert.Equal(t, "k8s-1.29", versions[0].Version)
	assert.True(t, versions[0].IsSupported)
	assert.False(t, versions[0].IsDeprecated)

	// Check extended support version
	assert.Equal(t, "k8s-1.28", versions[1].Version)
	assert.True(t, versions[1].IsSupported)
	assert.True(t, versions[1].IsExtendedSupport)

	// Check EOL version
	assert.Equal(t, "k8s-1.27", versions[2].Version)
	assert.False(t, versions[2].IsSupported)
	assert.True(t, versions[2].IsEOL)

	mockClient.AssertExpectations(t)
}

func TestEKSEOLProvider_Caching(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	future := now.AddDate(0, 6, 0)

	mockClient := new(MockEKSClient)
	provider := NewEKSEOLProvider(mockClient, time.Hour)

	mockVersions := []*EKSVersion{
		{
			KubernetesVersion: "1.29",
			Status:            "standard",
			EndOfStandardDate: &future,
		},
	}

	// First call should hit the API
	mockClient.On("DescribeAddonVersions", ctx).Return(mockVersions, nil).Once()

	// Make two calls
	versions1, err := provider.ListAllVersions(ctx, "kubernetes")
	require.NoError(t, err)

	versions2, err := provider.ListAllVersions(ctx, "kubernetes")
	require.NoError(t, err)

	// Both should return the same data
	assert.Equal(t, versions1, versions2)

	// Mock should have been called only once due to caching
	mockClient.AssertExpectations(t)
}
