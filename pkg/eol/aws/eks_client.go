package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/pkg/errors"
)

// RealEKSClient is the production implementation of EKSClient using AWS SDK v2
type RealEKSClient struct {
	client *eks.Client
	region string
}

// NewRealEKSClient creates a new real AWS EKS client
func NewRealEKSClient(ctx context.Context, region string) (*RealEKSClient, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, errors.Wrap(err, "failed to load AWS config")
	}

	return &RealEKSClient{
		client: eks.NewFromConfig(cfg),
		region: region,
	}, nil
}

// NewRealEKSClientFromConfig creates a new real AWS EKS client from an existing AWS config
func NewRealEKSClientFromConfig(cfg aws.Config) *RealEKSClient {
	return &RealEKSClient{
		client: eks.NewFromConfig(cfg),
		region: cfg.Region,
	}
}

// DescribeAddonVersions retrieves EKS addon version information from AWS
// This uses the DescribeAddonVersions API to get Kubernetes version lifecycle data
func (c *RealEKSClient) DescribeAddonVersions(ctx context.Context) ([]*EKSVersion, error) {
	// Call AWS EKS DescribeAddonVersions API
	// We'll use vpc-cni addon as it's available for all K8s versions
	// The API returns version compatibility info including K8s version lifecycle
	input := &eks.DescribeAddonVersionsInput{
		AddonName: aws.String("vpc-cni"),
	}

	var allVersions []*EKSVersion
	versionMap := make(map[string]*EKSVersion) // Deduplicate K8s versions

	// Handle pagination
	paginator := eks.NewDescribeAddonVersionsPaginator(c.client, input)
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to describe addon versions")
		}

		// Extract Kubernetes version information from addon compatibility
		for _, addon := range output.Addons {
			for _, addonVersion := range addon.AddonVersions {
				for _, compat := range addonVersion.Compatibilities {
					if compat.ClusterVersion == nil {
						continue
					}

					k8sVersion := aws.ToString(compat.ClusterVersion)
					if k8sVersion == "" {
						continue
					}

					// Skip if we've already seen this K8s version
					if _, exists := versionMap[k8sVersion]; exists {
						continue
					}

					// Create EKSVersion entry
					// Note: We don't use compat.DefaultVersion to determine K8s version support status
					// because it indicates whether this ADDON version is default, not whether the
					// K8s version is in standard support. A newer addon release could flip this to
					// false for the same K8s version, causing misclassification.
					//
					// Instead, status is determined by enrichWithKnownLifecycleDates() in eks.go,
					// which uses authoritative lifecycle dates from AWS documentation. The API data
					// here is supplementary (provides platform versions, etc).
					eksVersion := &EKSVersion{
						KubernetesVersion: k8sVersion,
						Status:            "", // Set by enrichWithKnownLifecycleDates based on actual dates
					}

					// Parse platform versions if available
					if len(compat.PlatformVersions) > 0 {
						eksVersion.LatestPlatformVersion = compat.PlatformVersions[0]
					}

					versionMap[k8sVersion] = eksVersion
				}
			}
		}
	}

	// Convert map to slice
	for _, version := range versionMap {
		allVersions = append(allVersions, version)
	}

	if len(allVersions) == 0 {
		return nil, errors.New("no EKS versions found from AWS API")
	}

	return allVersions, nil
}

// GetVersionLifecycleData fetches detailed version lifecycle data
// This is a helper method that attempts to get more precise EOL dates
// by querying additional AWS APIs or using known lifecycle data
func (c *RealEKSClient) GetVersionLifecycleData(ctx context.Context, k8sVersion string) (*EKSVersion, error) {
	// TODO: This could be enhanced to query actual EKS version lifecycle data
	// For now, we rely on DescribeAddonVersions
	versions, err := c.DescribeAddonVersions(ctx)
	if err != nil {
		return nil, err
	}

	for _, v := range versions {
		if v.KubernetesVersion == k8sVersion {
			return v, nil
		}
	}

	return nil, errors.Errorf("version %s not found", k8sVersion)
}
