package snapshot

import (
	"time"

	"github.com/google/uuid"

	"github.com/block/Version-Guard/pkg/types"
)

// Builder constructs a snapshot from findings
type Builder struct {
	snapshot *types.Snapshot
}

// NewBuilder creates a new snapshot builder
func NewBuilder() *Builder {
	return &Builder{
		snapshot: &types.Snapshot{
			SnapshotID:     uuid.New().String(),
			Version:        SnapshotSchemaVersion,
			GeneratedAt:    time.Now(),
			FindingsByType: make(map[types.ResourceType][]*types.Finding),
			Summary: types.SnapshotSummary{
				ByResourceType:  make(map[types.ResourceType]*types.StatBucket),
				ByService:       make(map[string]*types.StatBucket),
				ByCloudProvider: make(map[types.CloudProvider]*types.StatBucket),
			},
		},
	}
}

// WithScanTiming sets the scan timing information
func (b *Builder) WithScanTiming(startTime, endTime time.Time) *Builder {
	b.snapshot.ScanStartTime = startTime
	b.snapshot.ScanEndTime = endTime
	b.snapshot.ScanDurationSec = int64(endTime.Sub(startTime).Seconds())
	return b
}

// AddFindings adds findings for a specific resource type
func (b *Builder) AddFindings(resourceType types.ResourceType, findings []*types.Finding) *Builder {
	b.snapshot.FindingsByType[resourceType] = findings
	return b
}

// Build finalizes the snapshot and calculates all statistics
func (b *Builder) Build() *types.Snapshot {
	// Calculate aggregate statistics
	b.calculateStatistics()
	return b.snapshot
}

// calculateStatistics computes all summary statistics
func (b *Builder) calculateStatistics() {
	summary := &b.snapshot.Summary

	// Aggregate across all findings
	for resourceType, findings := range b.snapshot.FindingsByType {
		typeStat := &types.StatBucket{}

		for _, finding := range findings {
			// Overall counts
			summary.TotalResources++
			typeStat.TotalResources++
			incrementStatusCount(typeStat, finding.Status)
			incrementSummaryStatusCount(summary, finding.Status)

			// Service stats
			if finding.Service != "" {
				bucket := getOrCreate(summary.ByService, finding.Service)
				incrementStatusCount(bucket, finding.Status)
			}

			// Cloud provider stats
			bucket := getOrCreate(summary.ByCloudProvider, finding.CloudProvider)
			incrementStatusCount(bucket, finding.Status)
		}

		typeStat.CompliancePercentage = compliancePercentage(typeStat)
		summary.ByResourceType[resourceType] = typeStat
	}

	// Calculate overall compliance percentage
	if summary.TotalResources > 0 {
		summary.CompliancePercentage = (float64(summary.GreenCount) / float64(summary.TotalResources)) * 100
	}

	// Calculate compliance percentages for the keyed groupings.
	for _, bucket := range summary.ByService {
		bucket.CompliancePercentage = compliancePercentage(bucket)
	}
	for _, bucket := range summary.ByCloudProvider {
		bucket.CompliancePercentage = compliancePercentage(bucket)
	}
}

// getOrCreate returns the StatBucket stored under key, lazily inserting
// a fresh one (and bumping TotalResources) on the first observation.
// The map type is parameterised so it works for ByService (string keys),
// ByResourceType, and ByCloudProvider alike.
func getOrCreate[K comparable](m map[K]*types.StatBucket, key K) *types.StatBucket {
	bucket, ok := m[key]
	if !ok {
		bucket = &types.StatBucket{}
		m[key] = bucket
	}
	bucket.TotalResources++
	return bucket
}

// incrementStatusCount bumps the counter on a StatBucket that matches
// the given status. All grouping aggregations share this helper.
func incrementStatusCount(stat *types.StatBucket, status types.Status) {
	switch status {
	case types.StatusRed:
		stat.RedCount++
	case types.StatusYellow:
		stat.YellowCount++
	case types.StatusGreen:
		stat.GreenCount++
	case types.StatusUnknown:
		stat.UnknownCount++
	}
}

// incrementSummaryStatusCount bumps the top-level Red/Yellow/Green/Unknown
// counters on the SnapshotSummary itself. Kept separate from
// incrementStatusCount because SnapshotSummary holds these counts inline
// rather than in a StatBucket.
func incrementSummaryStatusCount(summary *types.SnapshotSummary, status types.Status) {
	switch status {
	case types.StatusRed:
		summary.RedCount++
	case types.StatusYellow:
		summary.YellowCount++
	case types.StatusGreen:
		summary.GreenCount++
	case types.StatusUnknown:
		summary.UnknownCount++
	}
}

// compliancePercentage returns the GreenCount/TotalResources percentage
// for a bucket, or 0 when the bucket is empty.
func compliancePercentage(stat *types.StatBucket) float64 {
	if stat.TotalResources == 0 {
		return 0
	}
	return (float64(stat.GreenCount) / float64(stat.TotalResources)) * 100
}
