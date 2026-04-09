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
				ByResourceType:  make(map[types.ResourceType]*types.TypeStat),
				ByService:       make(map[string]*types.ServiceStat),
				ByBrand:         make(map[string]*types.BrandStat),
				ByCloudProvider: make(map[types.CloudProvider]*types.CloudStat),
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
		typeStat := &types.TypeStat{}

		for _, finding := range findings {
			// Overall counts
			summary.TotalResources++
			typeStat.TotalResources++

			// Status counts
			switch finding.Status {
			case types.StatusRed:
				summary.RedCount++
				typeStat.RedCount++
			case types.StatusYellow:
				summary.YellowCount++
				typeStat.YellowCount++
			case types.StatusGreen:
				summary.GreenCount++
				typeStat.GreenCount++
			case types.StatusUnknown:
				summary.UnknownCount++
				typeStat.UnknownCount++
			}

			// Service stats
			if finding.Service != "" {
				if _, ok := summary.ByService[finding.Service]; !ok {
					summary.ByService[finding.Service] = &types.ServiceStat{}
				}
				serviceStat := summary.ByService[finding.Service]
				serviceStat.TotalResources++
				incrementStatusCount(serviceStat, finding.Status)
			}

			// Brand stats
			if finding.Brand != "" {
				if _, ok := summary.ByBrand[finding.Brand]; !ok {
					summary.ByBrand[finding.Brand] = &types.BrandStat{}
				}
				brandStat := summary.ByBrand[finding.Brand]
				brandStat.TotalResources++
				incrementBrandStatusCount(brandStat, finding.Status)
			}

			// Cloud provider stats
			if _, ok := summary.ByCloudProvider[finding.CloudProvider]; !ok {
				summary.ByCloudProvider[finding.CloudProvider] = &types.CloudStat{}
			}
			cloudStat := summary.ByCloudProvider[finding.CloudProvider]
			cloudStat.TotalResources++
			incrementCloudStatusCount(cloudStat, finding.Status)
		}

		// Calculate compliance percentage for this resource type
		if typeStat.TotalResources > 0 {
			typeStat.CompliancePercentage = (float64(typeStat.GreenCount) / float64(typeStat.TotalResources)) * 100
		}
		summary.ByResourceType[resourceType] = typeStat
	}

	// Calculate overall compliance percentage
	if summary.TotalResources > 0 {
		summary.CompliancePercentage = (float64(summary.GreenCount) / float64(summary.TotalResources)) * 100
	}

	// Calculate compliance percentages for services
	for _, serviceStat := range summary.ByService {
		if serviceStat.TotalResources > 0 {
			serviceStat.CompliancePercentage = (float64(serviceStat.GreenCount) / float64(serviceStat.TotalResources)) * 100
		}
	}

	// Calculate compliance percentages for brands
	for _, brandStat := range summary.ByBrand {
		if brandStat.TotalResources > 0 {
			brandStat.CompliancePercentage = (float64(brandStat.GreenCount) / float64(brandStat.TotalResources)) * 100
		}
	}

	// Calculate compliance percentages for cloud providers
	for _, cloudStat := range summary.ByCloudProvider {
		if cloudStat.TotalResources > 0 {
			cloudStat.CompliancePercentage = (float64(cloudStat.GreenCount) / float64(cloudStat.TotalResources)) * 100
		}
	}
}

func incrementStatusCount(stat *types.ServiceStat, status types.Status) {
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

func incrementBrandStatusCount(stat *types.BrandStat, status types.Status) {
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

func incrementCloudStatusCount(stat *types.CloudStat, status types.Status) {
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
