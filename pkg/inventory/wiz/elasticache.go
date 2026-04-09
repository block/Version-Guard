package wiz

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/pkg/errors"

	"github.com/block/Version-Guard/pkg/registry"
	"github.com/block/Version-Guard/pkg/types"
)

// ElastiCacheInventorySource fetches ElastiCache cluster inventory from Wiz saved reports
type ElastiCacheInventorySource struct {
	client         *Client
	reportID       string
	registryClient registry.Client // Optional: for service attribution when tags are missing
}

// NewElastiCacheInventorySource creates a new Wiz-based ElastiCache inventory source
func NewElastiCacheInventorySource(client *Client, reportID string) *ElastiCacheInventorySource {
	return &ElastiCacheInventorySource{
		client:   client,
		reportID: reportID,
	}
}

// WithRegistryClient adds optional registry integration for service attribution.
// When tags are missing, the registry will be queried to map AWS account → service.
func (s *ElastiCacheInventorySource) WithRegistryClient(registryClient registry.Client) *ElastiCacheInventorySource {
	s.registryClient = registryClient
	return s
}

// Name returns the name of this inventory source
func (s *ElastiCacheInventorySource) Name() string {
	return "wiz-elasticache"
}

// CloudProvider returns the cloud provider this source supports
func (s *ElastiCacheInventorySource) CloudProvider() types.CloudProvider {
	return types.CloudProviderAWS
}

// ListResources fetches all ElastiCache resources from Wiz
func (s *ElastiCacheInventorySource) ListResources(ctx context.Context, resourceType types.ResourceType) ([]*types.Resource, error) {
	if resourceType != types.ResourceTypeElastiCache {
		return nil, fmt.Errorf("unsupported resource type: %s (only ELASTICACHE supported)", resourceType)
	}

	// Fetch report data
	rows, err := s.client.GetReportData(ctx, s.reportID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch Wiz report data")
	}

	if len(rows) < 2 {
		// Empty report (only header row)
		return []*types.Resource{}, nil
	}

	// Skip header row, parse data rows
	var resources []*types.Resource
	for i, row := range rows[1:] {
		if len(row) < colMinRequired {
			// Skip malformed rows
			continue
		}

		// Filter for ElastiCache resources only
		nativeType := row[colNativeType]
		if !isElastiCacheResource(nativeType) {
			continue
		}

		resource, err := s.parseElastiCacheRow(ctx, row)
		if err != nil {
			// Log error but continue processing other rows
			// TODO: add proper logging
			_ = fmt.Sprintf("row %d: failed to parse ElastiCache resource: %v", i+1, err)
			continue
		}

		if resource != nil {
			resources = append(resources, resource)
		}
	}

	return resources, nil
}

// GetResource fetches a specific ElastiCache resource by ARN
func (s *ElastiCacheInventorySource) GetResource(ctx context.Context, resourceType types.ResourceType, id string) (*types.Resource, error) {
	// For Wiz source, we fetch all and filter
	resources, err := s.ListResources(ctx, resourceType)
	if err != nil {
		return nil, err
	}

	for _, resource := range resources {
		if resource.ID == id {
			return resource, nil
		}
	}

	return nil, fmt.Errorf("resource not found: %s", id)
}

// parseElastiCacheRow parses a single CSV row into a Resource
func (s *ElastiCacheInventorySource) parseElastiCacheRow(ctx context.Context, row []string) (*types.Resource, error) {
	resourceARN := row[colARN]
	if resourceARN == "" {
		return nil, fmt.Errorf("missing ARN")
	}

	// Parse ARN
	parsedARN, err := arn.Parse(resourceARN)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid ARN: %s", resourceARN)
	}

	// Extract metadata
	resourceName := row[colResourceName]
	accountID := row[colAWSAccountID]
	if accountID == "" {
		accountID = parsedARN.AccountID
	}

	engine := normalizeElastiCacheKind(row[colEngineKind])
	version := row[colEngineVersion]
	region := row[colRegion]

	// Parse tags
	tagsJSON := row[colTags]
	tags, err := ParseTags(tagsJSON)
	if err != nil {
		// Non-fatal, just use empty tags
		tags = make(map[string]string)
	}

	// Extract service name from tags
	service := GetTagValue(tags, AppTags)
	if service == "" {
		// Try registry lookup by AWS account (if registry is configured)
		if s.registryClient != nil {
			if serviceInfo, err := s.registryClient.GetServiceByAWSAccount(ctx, accountID, region); err == nil {
				service = serviceInfo.ServiceName
			}
			// Ignore registry errors - fall through to name parsing
		}

		// Final fallback: extract from resource name or ARN
		if service == "" {
			service = extractServiceFromName(resourceName)
		}
	}

	// Extract brand
	brand := GetTagValue(tags, BrandTags)

	resource := &types.Resource{
		ID:             resourceARN,
		Name:           resourceName,
		Type:           types.ResourceTypeElastiCache,
		CloudProvider:  types.CloudProviderAWS,
		Service:        service,
		CloudAccountID: accountID,
		CloudRegion:    region,
		Brand:          brand,
		CurrentVersion: version,
		Engine:         engine,
		Tags:           tags,
		DiscoveredAt:   time.Now(),
	}

	return resource, nil
}

// normalizeElastiCacheKind converts Wiz typeFields.kind values to standard engine names
// Wiz uses simple kind values: "Redis", "Memcached", "Valkey"
func normalizeElastiCacheKind(kind string) string {
	return strings.ToLower(kind)
}

// isElastiCacheResource checks if a Wiz native type represents an ElastiCache cluster or instance.
// Wiz nativeType examples:
//   - "elastiCache/Redis/cluster", "elastiCache/Redis/instance"
//   - "elastiCache/Memcached/cluster", "elastiCache/Valkey/instance"
//
// We exclude non-versioned types like "elasticache#snapshot", "elasticache#user", "elasticache#usergroup".
func isElastiCacheResource(nativeType string) bool {
	nativeType = strings.ToLower(nativeType)
	return strings.HasPrefix(nativeType, "elasticache/") &&
		(strings.HasSuffix(nativeType, "/cluster") || strings.HasSuffix(nativeType, "/instance"))
}
