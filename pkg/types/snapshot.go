package types

import "time"

// Snapshot represents a versioned point-in-time view of all findings
type Snapshot struct {
	// Metadata
	SnapshotID      string    `json:"snapshot_id"`
	Version         string    `json:"version"` // Schema version (e.g., "v1")
	GeneratedAt     time.Time `json:"generated_at"`
	ScanStartTime   time.Time `json:"scan_start_time"`
	ScanEndTime     time.Time `json:"scan_end_time"`
	ScanDurationSec int64     `json:"scan_duration_sec"`

	// Findings grouped by resource type
	FindingsByType map[ResourceType][]*Finding `json:"findings_by_type"`

	// Aggregate statistics
	Summary SnapshotSummary `json:"summary"`
}

// SnapshotSummary provides aggregate statistics across all resource types
type SnapshotSummary struct {
	TotalResources       int                          `json:"total_resources"`
	RedCount             int                          `json:"red_count"`
	YellowCount          int                          `json:"yellow_count"`
	GreenCount           int                          `json:"green_count"`
	UnknownCount         int                          `json:"unknown_count"`
	CompliancePercentage float64                      `json:"compliance_percentage"`
	ByResourceType       map[ResourceType]*TypeStat   `json:"by_resource_type"`
	ByService            map[string]*ServiceStat      `json:"by_service"`
	ByBrand              map[string]*BrandStat        `json:"by_brand"`
	ByCloudProvider      map[CloudProvider]*CloudStat `json:"by_cloud_provider"`
}

// TypeStat provides statistics for a specific resource type
type TypeStat struct {
	TotalResources       int     `json:"total_resources"`
	RedCount             int     `json:"red_count"`
	YellowCount          int     `json:"yellow_count"`
	GreenCount           int     `json:"green_count"`
	UnknownCount         int     `json:"unknown_count"`
	CompliancePercentage float64 `json:"compliance_percentage"`
}

// ServiceStat provides statistics for a specific service
type ServiceStat struct {
	TotalResources       int     `json:"total_resources"`
	RedCount             int     `json:"red_count"`
	YellowCount          int     `json:"yellow_count"`
	GreenCount           int     `json:"green_count"`
	UnknownCount         int     `json:"unknown_count"`
	CompliancePercentage float64 `json:"compliance_percentage"`
}

// BrandStat provides statistics for a specific brand
type BrandStat struct {
	TotalResources       int     `json:"total_resources"`
	RedCount             int     `json:"red_count"`
	YellowCount          int     `json:"yellow_count"`
	GreenCount           int     `json:"green_count"`
	UnknownCount         int     `json:"unknown_count"`
	CompliancePercentage float64 `json:"compliance_percentage"`
}

// CloudStat provides statistics for a specific cloud provider
type CloudStat struct {
	TotalResources       int     `json:"total_resources"`
	RedCount             int     `json:"red_count"`
	YellowCount          int     `json:"yellow_count"`
	GreenCount           int     `json:"green_count"`
	UnknownCount         int     `json:"unknown_count"`
	CompliancePercentage float64 `json:"compliance_percentage"`
}
