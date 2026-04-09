package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/block/Version-Guard/pkg/store"
	"github.com/block/Version-Guard/pkg/store/memory"
	"github.com/block/Version-Guard/pkg/types"
)

func setupTestService(_ *testing.T) (*Service, store.Store) {
	st := memory.NewStore()
	svc := NewService(st)
	return svc, st
}

func TestService_GetServiceScore_EmptyStore(t *testing.T) {
	svc, _ := setupTestService(t)

	req := &GetServiceScoreRequest{
		Service: "payments",
	}

	resp, err := svc.GetServiceScore(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, "payments", resp.Service)
	assert.Equal(t, ComplianceGradeBronze, resp.Grade) // Bronze by default (even if no resources)
	assert.Equal(t, 0, resp.TotalResources)
	assert.Equal(t, 0, resp.RedCount)
	assert.Equal(t, 0, resp.YellowCount)
	assert.Equal(t, 0, resp.GreenCount)
}

func TestService_GetServiceScore_AllGreen(t *testing.T) {
	svc, st := setupTestService(t)

	// Add some GREEN findings
	findings := []*types.Finding{
		{
			ResourceID:   "arn:aws:rds:us-east-1:123:cluster:db-1",
			ResourceName: "db-1",
			Service:      "payments",
			Status:       types.StatusGreen,
			DetectedAt:   time.Now(),
			UpdatedAt:    time.Now(),
		},
		{
			ResourceID:   "arn:aws:rds:us-east-1:123:cluster:db-2",
			ResourceName: "db-2",
			Service:      "payments",
			Status:       types.StatusGreen,
			DetectedAt:   time.Now(),
			UpdatedAt:    time.Now(),
		},
	}

	err := st.SaveFindings(context.Background(), findings)
	require.NoError(t, err)

	req := &GetServiceScoreRequest{
		Service: "payments",
	}

	resp, err := svc.GetServiceScore(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, "payments", resp.Service)
	assert.Equal(t, ComplianceGradeGold, resp.Grade) // Gold: no RED or YELLOW
	assert.Equal(t, 2, resp.TotalResources)
	assert.Equal(t, 0, resp.RedCount)
	assert.Equal(t, 0, resp.YellowCount)
	assert.Equal(t, 2, resp.GreenCount)
	assert.Equal(t, float32(100.0), resp.CompliancePercentage)
}

func TestService_GetServiceScore_SomeYellow(t *testing.T) {
	svc, st := setupTestService(t)

	// Add GREEN + YELLOW findings
	findings := []*types.Finding{
		{
			ResourceID: "arn:aws:rds:us-east-1:123:cluster:db-1",
			Service:    "payments",
			Status:     types.StatusGreen,
			DetectedAt: time.Now(),
			UpdatedAt:  time.Now(),
		},
		{
			ResourceID: "arn:aws:rds:us-east-1:123:cluster:db-2",
			Service:    "payments",
			Status:     types.StatusYellow,
			DetectedAt: time.Now(),
			UpdatedAt:  time.Now(),
		},
	}

	err := st.SaveFindings(context.Background(), findings)
	require.NoError(t, err)

	req := &GetServiceScoreRequest{
		Service: "payments",
	}

	resp, err := svc.GetServiceScore(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, "payments", resp.Service)
	assert.Equal(t, ComplianceGradeSilver, resp.Grade) // Silver: no RED but has YELLOW
	assert.Equal(t, 2, resp.TotalResources)
	assert.Equal(t, 0, resp.RedCount)
	assert.Equal(t, 1, resp.YellowCount)
	assert.Equal(t, 1, resp.GreenCount)
	assert.Equal(t, float32(50.0), resp.CompliancePercentage)
}

func TestService_GetServiceScore_SomeRed(t *testing.T) {
	svc, st := setupTestService(t)

	// Add RED + YELLOW + GREEN findings
	findings := []*types.Finding{
		{
			ResourceID: "arn:aws:rds:us-east-1:123:cluster:db-1",
			Service:    "payments",
			Status:     types.StatusGreen,
			DetectedAt: time.Now(),
			UpdatedAt:  time.Now(),
		},
		{
			ResourceID: "arn:aws:rds:us-east-1:123:cluster:db-2",
			Service:    "payments",
			Status:     types.StatusYellow,
			DetectedAt: time.Now(),
			UpdatedAt:  time.Now(),
		},
		{
			ResourceID: "arn:aws:rds:us-east-1:123:cluster:db-3",
			Service:    "payments",
			Status:     types.StatusRed,
			DetectedAt: time.Now(),
			UpdatedAt:  time.Now(),
		},
	}

	err := st.SaveFindings(context.Background(), findings)
	require.NoError(t, err)

	req := &GetServiceScoreRequest{
		Service: "payments",
	}

	resp, err := svc.GetServiceScore(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, "payments", resp.Service)
	assert.Equal(t, ComplianceGradeBronze, resp.Grade) // Bronze: has RED
	assert.Equal(t, 3, resp.TotalResources)
	assert.Equal(t, 1, resp.RedCount)
	assert.Equal(t, 1, resp.YellowCount)
	assert.Equal(t, 1, resp.GreenCount)
	assert.InDelta(t, 33.33, resp.CompliancePercentage, 0.01)
}

func TestService_ListFindings_NoFilters(t *testing.T) {
	svc, st := setupTestService(t)

	// Add findings
	now := time.Now()
	findings := []*types.Finding{
		{
			ResourceID:     "arn:aws:rds:us-east-1:123:cluster:db-1",
			ResourceName:   "db-1",
			Service:        "payments",
			Status:         types.StatusRed,
			Message:        "Version past EOL",
			Recommendation: "Upgrade immediately",
			DetectedAt:     now,
			UpdatedAt:      now,
		},
		{
			ResourceID:   "arn:aws:rds:us-east-1:123:cluster:db-2",
			ResourceName: "db-2",
			Service:      "billing",
			Status:       types.StatusGreen,
			DetectedAt:   now,
			UpdatedAt:    now,
		},
	}

	err := st.SaveFindings(context.Background(), findings)
	require.NoError(t, err)

	req := &ListFindingsRequest{}

	resp, err := svc.ListFindings(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, 2, resp.TotalCount)
	assert.Len(t, resp.Findings, 2)
}

func TestService_ListFindings_FilterByService(t *testing.T) {
	svc, st := setupTestService(t)

	// Add findings for different services
	findings := []*types.Finding{
		{
			ResourceID: "arn:aws:rds:us-east-1:123:cluster:db-1",
			Service:    "payments",
			Status:     types.StatusRed,
			DetectedAt: time.Now(),
			UpdatedAt:  time.Now(),
		},
		{
			ResourceID: "arn:aws:rds:us-east-1:123:cluster:db-2",
			Service:    "billing",
			Status:     types.StatusGreen,
			DetectedAt: time.Now(),
			UpdatedAt:  time.Now(),
		},
		{
			ResourceID: "arn:aws:rds:us-east-1:123:cluster:db-3",
			Service:    "payments",
			Status:     types.StatusYellow,
			DetectedAt: time.Now(),
			UpdatedAt:  time.Now(),
		},
	}

	err := st.SaveFindings(context.Background(), findings)
	require.NoError(t, err)

	service := "payments"
	req := &ListFindingsRequest{
		Service: &service,
	}

	resp, err := svc.ListFindings(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, 2, resp.TotalCount) // Only payments
	assert.Len(t, resp.Findings, 2)
	for _, f := range resp.Findings {
		assert.Equal(t, "payments", f.Service)
	}
}

func TestService_ListFindings_FilterByStatus(t *testing.T) {
	svc, st := setupTestService(t)

	// Add findings with different statuses
	findings := []*types.Finding{
		{
			ResourceID: "arn:aws:rds:us-east-1:123:cluster:db-1",
			Service:    "payments",
			Status:     types.StatusRed,
			DetectedAt: time.Now(),
			UpdatedAt:  time.Now(),
		},
		{
			ResourceID: "arn:aws:rds:us-east-1:123:cluster:db-2",
			Service:    "payments",
			Status:     types.StatusGreen,
			DetectedAt: time.Now(),
			UpdatedAt:  time.Now(),
		},
		{
			ResourceID: "arn:aws:rds:us-east-1:123:cluster:db-3",
			Service:    "payments",
			Status:     types.StatusRed,
			DetectedAt: time.Now(),
			UpdatedAt:  time.Now(),
		},
	}

	err := st.SaveFindings(context.Background(), findings)
	require.NoError(t, err)

	statusFilter := StatusRed
	req := &ListFindingsRequest{
		Status: &statusFilter,
	}

	resp, err := svc.ListFindings(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, 2, resp.TotalCount) // Only RED
	assert.Len(t, resp.Findings, 2)
	for _, f := range resp.Findings {
		assert.Equal(t, StatusRed, f.Status)
	}
}

func TestService_ListFindings_WithLimit(t *testing.T) {
	svc, st := setupTestService(t)

	// Add many findings
	findings := make([]*types.Finding, 50)
	for i := 0; i < 50; i++ {
		findings[i] = &types.Finding{
			ResourceID: "arn:aws:rds:us-east-1:123:cluster:db-" + string(rune(i)),
			Service:    "payments",
			Status:     types.StatusGreen,
			DetectedAt: time.Now(),
			UpdatedAt:  time.Now(),
		}
	}

	err := st.SaveFindings(context.Background(), findings)
	require.NoError(t, err)

	req := &ListFindingsRequest{
		Limit: 10,
	}

	resp, err := svc.ListFindings(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, 10, resp.TotalCount) // Limited to 10
	assert.Len(t, resp.Findings, 10)
}

func TestService_GetFleetSummary(t *testing.T) {
	svc, st := setupTestService(t)

	// Add findings across multiple services and brands
	now := time.Now()
	findings := []*types.Finding{
		{
			ResourceID:    "arn:aws:rds:us-east-1:123:cluster:db-1",
			Service:       "payments",
			Brand:         "brand-a",
			CloudProvider: types.CloudProviderAWS,
			Status:        types.StatusRed,
			DetectedAt:    now,
			UpdatedAt:     now,
		},
		{
			ResourceID:    "arn:aws:rds:us-east-1:123:cluster:db-2",
			Service:       "payments",
			Brand:         "brand-a",
			CloudProvider: types.CloudProviderAWS,
			Status:        types.StatusYellow,
			DetectedAt:    now.Add(-time.Hour),
			UpdatedAt:     now.Add(-time.Hour),
		},
		{
			ResourceID:    "arn:aws:rds:us-east-1:123:cluster:db-3",
			Service:       "billing",
			Brand:         "brand-b",
			CloudProvider: types.CloudProviderAWS,
			Status:        types.StatusGreen,
			DetectedAt:    now.Add(-2 * time.Hour),
			UpdatedAt:     now.Add(-2 * time.Hour),
		},
	}

	err := st.SaveFindings(context.Background(), findings)
	require.NoError(t, err)

	req := &GetFleetSummaryRequest{}

	resp, err := svc.GetFleetSummary(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, 3, resp.TotalResources)
	assert.Equal(t, 1, resp.RedCount)
	assert.Equal(t, 1, resp.YellowCount)
	assert.Equal(t, 1, resp.GreenCount)
	assert.InDelta(t, 33.33, resp.CompliancePercentage, 0.01)

	// Check last scan time (should be the latest)
	require.NotNil(t, resp.LastScan)
	assert.True(t, resp.LastScan.Equal(now) || resp.LastScan.After(now.Add(-time.Second)))

	// Check breakdowns
	assert.Equal(t, 2, resp.ByService["payments"])
	assert.Equal(t, 1, resp.ByService["billing"])
	assert.Equal(t, 2, resp.ByBrand["brand-a"])
	assert.Equal(t, 1, resp.ByBrand["brand-b"])
	assert.Equal(t, 3, resp.ByCloudProvider["aws"])
}
