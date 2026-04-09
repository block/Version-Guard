package types

import "time"

// ResourceType represents the type of cloud resource
type ResourceType string

const (
	// ResourceTypeAurora represents AWS Aurora RDS clusters
	ResourceTypeAurora ResourceType = "AURORA"
	// ResourceTypeElastiCache represents AWS ElastiCache clusters
	ResourceTypeElastiCache ResourceType = "ELASTICACHE"
	// ResourceTypeOpenSearch represents AWS OpenSearch clusters
	ResourceTypeOpenSearch ResourceType = "OPENSEARCH"
	// ResourceTypeEKS represents AWS Elastic Kubernetes Service clusters
	ResourceTypeEKS ResourceType = "EKS"
	// ResourceTypeLambda represents AWS Lambda functions
	ResourceTypeLambda ResourceType = "LAMBDA"
	// ResourceTypeCloudSQL represents GCP Cloud SQL instances (future)
	ResourceTypeCloudSQL ResourceType = "CLOUDSQL"
	// ResourceTypeMemorystore represents GCP Memorystore instances (future)
	ResourceTypeMemorystore ResourceType = "MEMORYSTORE"
	// ResourceTypeGKE represents GCP Google Kubernetes Engine clusters (future)
	ResourceTypeGKE ResourceType = "GKE"
)

// String returns the string representation of the ResourceType
func (r ResourceType) String() string {
	return string(r)
}

// Resource represents a cloud infrastructure resource with version information
type Resource struct {
	// Tags are key-value pairs associated with the resource
	Tags map[string]string

	// DiscoveredAt is the timestamp when this resource was discovered
	DiscoveredAt time.Time

	// ID is the cloud-specific resource identifier (ARN for AWS, resource path for GCP, etc.)
	ID string

	// Name is the human-readable name of the resource
	Name string

	// Service is the application or service name that owns this resource
	Service string

	// CloudAccountID is the cloud account identifier
	// - AWS: Account ID (e.g., "123456789012")
	// - GCP: Project ID (e.g., "my-project-123")
	// - Azure: Subscription ID (e.g., "12345678-1234-1234-1234-123456789012")
	CloudAccountID string

	// CloudRegion is the cloud region where the resource is deployed
	// - AWS: Region (e.g., "us-east-1")
	// - GCP: Region (e.g., "us-central1")
	// - Azure: Region (e.g., "eastus")
	CloudRegion string

	// Brand is the business unit or brand (e.g., "brand-a", "brand-b")
	Brand string

	// CurrentVersion is the engine or runtime version currently running
	CurrentVersion string

	// Engine is the database/cache/runtime engine type (e.g., "aurora-mysql", "postgres", "redis")
	Engine string

	// Type is the type of resource (Aurora, CloudSQL, etc.)
	Type ResourceType

	// CloudProvider is the cloud platform hosting this resource (AWS, GCP, Azure)
	CloudProvider CloudProvider
}

// VersionLifecycle represents the lifecycle information for a specific version
type VersionLifecycle struct {
	// ReleaseDate is when this version was released
	ReleaseDate *time.Time

	// EOLDate is when this version reaches End-of-Life
	EOLDate *time.Time

	// ExtendedSupportEnd is when extended support ends (if applicable)
	ExtendedSupportEnd *time.Time

	// DeprecationDate is when this version is deprecated
	DeprecationDate *time.Time

	// FetchedAt is the timestamp when this lifecycle data was fetched
	FetchedAt time.Time

	// Version is the version string (e.g., "5.6.10a", "7.0.1")
	Version string

	// Engine is the engine type (e.g., "aurora-mysql", "postgres")
	Engine string

	// Source indicates where this lifecycle data came from (e.g., "aws-rds-api", "endoflife.date")
	Source string

	// IsEOL indicates if the version is past End-of-Life
	IsEOL bool

	// IsDeprecated indicates if the version is deprecated
	IsDeprecated bool

	// IsExtendedSupport indicates if the version is in extended support (typically higher cost)
	IsExtendedSupport bool

	// IsSupported indicates if the version is currently supported
	IsSupported bool
}

// Finding represents a detected version drift issue
type Finding struct {
	// EOLDate is when the current version reaches End-of-Life
	EOLDate *time.Time

	// DetectedAt is when this finding was first detected
	DetectedAt time.Time

	// UpdatedAt is when this finding was last updated
	UpdatedAt time.Time

	// ResourceID is the cloud-specific resource identifier
	ResourceID string

	// ResourceName is the human-readable name of the resource
	ResourceName string

	// Service is the application or service name
	Service string

	// CloudAccountID is the cloud account identifier
	CloudAccountID string

	// CloudRegion is the cloud region
	CloudRegion string

	// Brand is the business unit or brand
	Brand string

	// CurrentVersion is the version currently running
	CurrentVersion string

	// Engine is the engine type
	Engine string

	// Message is a human-readable description of the issue
	Message string

	// Recommendation is the recommended action to resolve the issue
	Recommendation string

	// ResourceType is the type of resource
	ResourceType ResourceType

	// CloudProvider is the cloud platform
	CloudProvider CloudProvider

	// Status is the compliance status (RED/YELLOW/GREEN/UNKNOWN)
	Status Status
}

// ScanSummary provides aggregate statistics from a scan
//
//nolint:govet // field alignment sacrificed for readability
type ScanSummary struct {
	// ScanStartTime is when the scan started
	ScanStartTime time.Time

	// ScanEndTime is when the scan completed
	ScanEndTime time.Time

	// CompliancePercentage is the percentage of resources that are compliant (GREEN)
	CompliancePercentage float64

	// TotalResources is the total number of resources scanned
	TotalResources int

	// RedCount is the number of resources with RED status
	RedCount int

	// YellowCount is the number of resources with YELLOW status
	YellowCount int

	// GreenCount is the number of resources with GREEN status
	GreenCount int

	// UnknownCount is the number of resources with UNKNOWN status
	UnknownCount int

	// ResourceType is the type of resources scanned
	ResourceType ResourceType

	// CloudProvider is the cloud provider scanned
	CloudProvider CloudProvider
}
