package orchestrator

import (
	"context"
	"time"

	"go.temporal.io/sdk/activity"

	"github.com/block/Version-Guard/pkg/snapshot"
	"github.com/block/Version-Guard/pkg/store"
	"github.com/block/Version-Guard/pkg/types"
)

// Activity names
const (
	RetrieveFindingsActivityName = "version-guard.RetrieveFindings"
	CreateSnapshotActivityName   = "version-guard.CreateSnapshot"
)

// Activity input/output types

type RetrieveFindingsInput struct {
	ResourceType types.ResourceType
}

type CreateSnapshotInput struct {
	ScanID         string
	FindingsByType map[types.ResourceType][]*types.Finding
	ScanStartTime  time.Time
	ScanEndTime    time.Time
}

type SnapshotResult struct {
	SnapshotID           string
	TotalFindings        int
	CompliancePercentage float64
}

type SignalActWorkflowInput struct {
	SnapshotID string
}

// Activities struct holds dependencies
type Activities struct {
	Store         store.Store
	SnapshotStore snapshot.Store
}

// NewActivities creates a new Activities instance
func NewActivities(
	store store.Store,
	snapshotStore snapshot.Store,
) *Activities {
	return &Activities{
		Store:         store,
		SnapshotStore: snapshotStore,
	}
}

// RetrieveFindings retrieves all findings for a given resource type from the store
func (a *Activities) RetrieveFindings(ctx context.Context, input RetrieveFindingsInput) ([]*types.Finding, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Retrieving findings from store", "resourceType", input.ResourceType)

	filters := store.FindingFilters{
		ResourceType: &input.ResourceType,
	}

	findings, err := a.Store.ListFindings(ctx, filters)
	if err != nil {
		return nil, err
	}

	logger.Info("Findings retrieved", "count", len(findings))
	return findings, nil
}

// CreateSnapshot creates a snapshot from findings and persists it to S3
func (a *Activities) CreateSnapshot(ctx context.Context, input CreateSnapshotInput) (*SnapshotResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Creating snapshot", "scanID", input.ScanID, "resourceTypeCount", len(input.FindingsByType))

	// Build snapshot
	builder := snapshot.NewBuilder()
	builder.WithScanTiming(input.ScanStartTime, input.ScanEndTime)

	for resourceType, findings := range input.FindingsByType {
		builder.AddFindings(resourceType, findings)
	}

	snap := builder.Build()
	snap.SnapshotID = input.ScanID // Use scan ID as snapshot ID for correlation

	// Persist to S3
	err := a.SnapshotStore.SaveSnapshot(ctx, snap)
	if err != nil {
		return nil, err
	}

	logger.Info("Snapshot created and persisted",
		"snapshotID", snap.SnapshotID,
		"totalFindings", snap.Summary.TotalResources,
		"compliance", snap.Summary.CompliancePercentage)

	return &SnapshotResult{
		SnapshotID:           snap.SnapshotID,
		TotalFindings:        snap.Summary.TotalResources,
		CompliancePercentage: snap.Summary.CompliancePercentage,
	}, nil
}

// RegisterActivities registers all activities with a Temporal worker
func RegisterActivities(worker interface {
	RegisterActivityWithOptions(interface{}, activity.RegisterOptions)
}, activities *Activities) {
	worker.RegisterActivityWithOptions(activities.RetrieveFindings, activity.RegisterOptions{
		Name: RetrieveFindingsActivityName,
	})
	worker.RegisterActivityWithOptions(activities.CreateSnapshot, activity.RegisterOptions{
		Name: CreateSnapshotActivityName,
	})
}
