package grpc

import (
	"time"

	"github.com/block/Version-Guard/pkg/types"
)

// ResourceType represents the type of infrastructure resource
type ResourceType string

const (
	ResourceTypeUnspecified ResourceType = ""
	ResourceTypeAurora      ResourceType = "aurora"
	ResourceTypeElastiCache ResourceType = "elasticache"
	ResourceTypeOpenSearch  ResourceType = "opensearch"
	ResourceTypeEKS         ResourceType = "eks"
	ResourceTypeLambda      ResourceType = "lambda"
)

// CloudProvider represents the cloud provider
type CloudProvider string

const (
	CloudProviderUnspecified CloudProvider = ""
	CloudProviderAWS         CloudProvider = "aws"
	CloudProviderGCP         CloudProvider = "gcp"
	CloudProviderAzure       CloudProvider = "azure"
)

// Status represents the compliance status
type Status string

const (
	StatusUnspecified Status = ""
	StatusGreen       Status = "green"
	StatusYellow      Status = "yellow"
	StatusRed         Status = "red"
	StatusUnknown     Status = "unknown"
)

// ComplianceGrade represents the overall compliance grade
type ComplianceGrade string

const (
	ComplianceGradeUnspecified ComplianceGrade = ""
	ComplianceGradeBronze      ComplianceGrade = "bronze"
	ComplianceGradeSilver      ComplianceGrade = "silver"
	ComplianceGradeGold        ComplianceGrade = "gold"
)

// Finding represents a version drift finding (API response type)
//
//nolint:govet // field alignment sacrificed for JSON field grouping
type Finding struct {
	ResourceID     string        `json:"resource_id"`
	ResourceName   string        `json:"resource_name"`
	ResourceType   ResourceType  `json:"resource_type"`
	CloudProvider  CloudProvider `json:"cloud_provider"`
	Service        string        `json:"service"`
	CloudAccountID string        `json:"cloud_account_id"`
	CloudRegion    string        `json:"cloud_region"`
	Brand          string        `json:"brand"`
	CurrentVersion string        `json:"current_version"`
	Engine         string        `json:"engine"`
	Status         Status        `json:"status"`
	Message        string        `json:"message"`
	Recommendation string        `json:"recommendation"`
	EOLDate        *time.Time    `json:"eol_date,omitempty"`
	DetectedAt     time.Time     `json:"detected_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
}

// GetServiceScoreRequest requests compliance score for a service
//
//nolint:govet // field alignment sacrificed for JSON field grouping
type GetServiceScoreRequest struct {
	Service       string         `json:"service"`
	ResourceType  *ResourceType  `json:"resource_type,omitempty"`
	CloudProvider *CloudProvider `json:"cloud_provider,omitempty"`
}

// GetServiceScoreResponse returns compliance score and grade
type GetServiceScoreResponse struct {
	Service              string          `json:"service"`
	Grade                ComplianceGrade `json:"grade"`
	TotalResources       int             `json:"total_resources"`
	RedCount             int             `json:"red_count"`
	YellowCount          int             `json:"yellow_count"`
	GreenCount           int             `json:"green_count"`
	UnknownCount         int             `json:"unknown_count"`
	CompliancePercentage float32         `json:"compliance_percentage"`
}

// ListFindingsRequest requests findings with filters
type ListFindingsRequest struct {
	CloudProvider  *CloudProvider `json:"cloud_provider,omitempty"`
	ResourceType   *ResourceType  `json:"resource_type,omitempty"`
	Service        *string        `json:"service,omitempty"`
	Status         *Status        `json:"status,omitempty"`
	Brand          *string        `json:"brand,omitempty"`
	CloudAccountID *string        `json:"cloud_account_id,omitempty"`
	CloudRegion    *string        `json:"cloud_region,omitempty"`
	Limit          int            `json:"limit,omitempty"`
}

// ListFindingsResponse returns list of findings
type ListFindingsResponse struct {
	Findings   []Finding `json:"findings"`
	TotalCount int       `json:"total_count"`
}

// GetFleetSummaryRequest requests fleet-wide statistics
type GetFleetSummaryRequest struct {
	CloudProvider *CloudProvider `json:"cloud_provider,omitempty"`
	ResourceType  *ResourceType  `json:"resource_type,omitempty"`
}

// GetFleetSummaryResponse returns fleet-wide statistics
//
//nolint:govet // field alignment sacrificed for JSON field grouping
type GetFleetSummaryResponse struct {
	TotalResources       int            `json:"total_resources"`
	RedCount             int            `json:"red_count"`
	YellowCount          int            `json:"yellow_count"`
	GreenCount           int            `json:"green_count"`
	UnknownCount         int            `json:"unknown_count"`
	CompliancePercentage float32        `json:"compliance_percentage"`
	LastScan             *time.Time     `json:"last_scan,omitempty"`
	ByService            map[string]int `json:"by_service"`
	ByBrand              map[string]int `json:"by_brand"`
	ByCloudProvider      map[string]int `json:"by_cloud_provider"`
}

// Convert internal types to API types

func toAPIResourceType(rt types.ResourceType) ResourceType {
	switch rt {
	case types.ResourceTypeAurora:
		return ResourceTypeAurora
	case types.ResourceTypeElastiCache:
		return ResourceTypeElastiCache
	case types.ResourceTypeOpenSearch:
		return ResourceTypeOpenSearch
	case types.ResourceTypeEKS:
		return ResourceTypeEKS
	case types.ResourceTypeLambda:
		return ResourceTypeLambda
	default:
		return ResourceTypeUnspecified
	}
}

func toAPICloudProvider(cp types.CloudProvider) CloudProvider {
	switch cp {
	case types.CloudProviderAWS:
		return CloudProviderAWS
	case types.CloudProviderGCP:
		return CloudProviderGCP
	case types.CloudProviderAzure:
		return CloudProviderAzure
	default:
		return CloudProviderUnspecified
	}
}

func toAPIStatus(s types.Status) Status {
	switch s {
	case types.StatusGreen:
		return StatusGreen
	case types.StatusYellow:
		return StatusYellow
	case types.StatusRed:
		return StatusRed
	case types.StatusUnknown:
		return StatusUnknown
	default:
		return StatusUnspecified
	}
}

func toInternalStatus(s *Status) *types.Status {
	if s == nil {
		return nil
	}
	var result types.Status
	switch *s {
	case StatusGreen:
		result = types.StatusGreen
	case StatusYellow:
		result = types.StatusYellow
	case StatusRed:
		result = types.StatusRed
	case StatusUnknown:
		result = types.StatusUnknown
	default:
		return nil
	}
	return &result
}

func toAPIFinding(f *types.Finding) Finding {
	return Finding{
		ResourceID:     f.ResourceID,
		ResourceName:   f.ResourceName,
		ResourceType:   toAPIResourceType(f.ResourceType),
		CloudProvider:  toAPICloudProvider(f.CloudProvider),
		Service:        f.Service,
		CloudAccountID: f.CloudAccountID,
		CloudRegion:    f.CloudRegion,
		Brand:          f.Brand,
		CurrentVersion: f.CurrentVersion,
		Engine:         f.Engine,
		Status:         toAPIStatus(f.Status),
		Message:        f.Message,
		Recommendation: f.Recommendation,
		EOLDate:        f.EOLDate,
		DetectedAt:     f.DetectedAt,
		UpdatedAt:      f.UpdatedAt,
	}
}
