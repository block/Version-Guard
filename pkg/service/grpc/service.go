package grpc

import (
	"context"
	"time"

	"github.com/block/Version-Guard/pkg/store"
	"github.com/block/Version-Guard/pkg/types"
)

// Service implements the Version Guard gRPC service
type Service struct {
	store store.Store
}

// NewService creates a new Version Guard service
func NewService(store store.Store) *Service {
	return &Service{
		store: store,
	}
}

// GetServiceScore returns compliance score for a specific service
func (s *Service) GetServiceScore(ctx context.Context, req *GetServiceScoreRequest) (*GetServiceScoreResponse, error) {
	// Build filters
	filters := store.FindingFilters{
		Service: &req.Service,
	}

	// TODO: Add resource type and cloud provider filtering when store supports it

	// Get findings from store
	findings, err := s.store.ListFindings(ctx, filters)
	if err != nil {
		return nil, err
	}

	// Count by status
	var red, yellow, green, unknown int
	for _, f := range findings {
		switch f.Status {
		case types.StatusRed:
			red++
		case types.StatusYellow:
			yellow++
		case types.StatusGreen:
			green++
		case types.StatusUnknown:
			unknown++
		}
	}

	total := len(findings)

	// Calculate compliance grade
	grade := ComplianceGradeBronze // Default: data exists
	if total > 0 {
		if red == 0 {
			grade = ComplianceGradeSilver // No critical issues
		}
		if red == 0 && yellow == 0 {
			grade = ComplianceGradeGold // Fully compliant
		}
	}

	// Calculate compliance percentage
	var compliancePercentage float32
	if total > 0 {
		compliancePercentage = float32(green) / float32(total) * 100
	}

	return &GetServiceScoreResponse{
		Service:              req.Service,
		Grade:                grade,
		TotalResources:       total,
		RedCount:             red,
		YellowCount:          yellow,
		GreenCount:           green,
		UnknownCount:         unknown,
		CompliancePercentage: compliancePercentage,
	}, nil
}

// ListFindings returns findings with optional filters
func (s *Service) ListFindings(ctx context.Context, req *ListFindingsRequest) (*ListFindingsResponse, error) {
	// Build filters
	filters := store.FindingFilters{
		Service:        req.Service,
		Brand:          req.Brand,
		CloudAccountID: req.CloudAccountID,
		CloudRegion:    req.CloudRegion,
	}

	// Convert API status to internal status
	if req.Status != nil {
		internalStatus := toInternalStatus(req.Status)
		filters.Status = internalStatus
	}

	// TODO: Add resource type and cloud provider filtering when store supports it

	// Get findings from store
	findings, err := s.store.ListFindings(ctx, filters)
	if err != nil {
		return nil, err
	}

	// Apply limit if specified
	if req.Limit > 0 && len(findings) > req.Limit {
		findings = findings[:req.Limit]
	}

	// Convert to API findings
	apiFindings := make([]Finding, len(findings))
	for i, f := range findings {
		apiFindings[i] = toAPIFinding(f)
	}

	return &ListFindingsResponse{
		Findings:   apiFindings,
		TotalCount: len(apiFindings),
	}, nil
}

// GetFleetSummary returns aggregate fleet-wide statistics
func (s *Service) GetFleetSummary(ctx context.Context, req *GetFleetSummaryRequest) (*GetFleetSummaryResponse, error) {
	// Build filters
	filters := store.FindingFilters{}

	// TODO: Add resource type and cloud provider filtering when store supports it

	// Get all findings
	findings, err := s.store.ListFindings(ctx, filters)
	if err != nil {
		return nil, err
	}

	// Calculate aggregate statistics
	var red, yellow, green, unknown int
	byService := make(map[string]int)
	byBrand := make(map[string]int)
	byCloudProvider := make(map[string]int)

	var lastScan *time.Time

	for _, f := range findings {
		// Count by status
		switch f.Status {
		case types.StatusRed:
			red++
		case types.StatusYellow:
			yellow++
		case types.StatusGreen:
			green++
		case types.StatusUnknown:
			unknown++
		}

		// Count by service
		byService[f.Service]++

		// Count by brand
		if f.Brand != "" {
			byBrand[f.Brand]++
		}

		// Count by cloud provider (convert to lowercase for consistency)
		if f.CloudProvider != "" {
			cpKey := string(f.CloudProvider)
			// Convert to lowercase for map key
			if cpKey == "AWS" {
				cpKey = "aws"
			} else if cpKey == "GCP" {
				cpKey = "gcp"
			} else if cpKey == "AZURE" {
				cpKey = "azure"
			}
			byCloudProvider[cpKey]++
		}

		// Track latest scan time
		if lastScan == nil || f.DetectedAt.After(*lastScan) {
			ts := f.DetectedAt
			lastScan = &ts
		}
	}

	total := len(findings)

	// Calculate compliance percentage
	var compliancePercentage float32
	if total > 0 {
		compliancePercentage = float32(green) / float32(total) * 100
	}

	return &GetFleetSummaryResponse{
		TotalResources:       total,
		RedCount:             red,
		YellowCount:          yellow,
		GreenCount:           green,
		UnknownCount:         unknown,
		CompliancePercentage: compliancePercentage,
		LastScan:             lastScan,
		ByService:            byService,
		ByBrand:              byBrand,
		ByCloudProvider:      byCloudProvider,
	}, nil
}
