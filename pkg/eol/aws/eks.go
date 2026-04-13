package aws

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/sync/singleflight"

	"github.com/block/Version-Guard/pkg/eol/endoflife"
	"github.com/block/Version-Guard/pkg/types"
)

// EKSClient defines the interface for AWS EKS API calls
// This allows us to mock the AWS SDK for testing
type EKSClient interface {
	// DescribeAddonVersions retrieves EKS addon version information
	// Used to determine supported Kubernetes versions
	DescribeAddonVersions(ctx context.Context) ([]*EKSVersion, error)
}

type EKSVersion struct {
	KubernetesVersion     string     // e.g., "1.28", "1.29"
	Status                string     // "standard", "extended", "deprecated"
	ReleaseDate           *time.Time // When this version was released
	EndOfStandardDate     *time.Time // End of standard support
	EndOfExtendedDate     *time.Time // End of extended support
	LatestPlatformVersion string     // Latest platform version for this K8s version
}

// EKS version status constants
const (
	eksStatusStandard   = "standard"
	eksStatusExtended   = "extended"
	eksStatusDeprecated = "deprecated"
)

// EKSEOLProvider fetches EOL data from AWS EKS API
//
//nolint:govet // field alignment sacrificed for readability
type EKSEOLProvider struct {
	mu              sync.RWMutex
	cache           map[string]*cachedEKSVersions
	client          EKSClient
	endoflifeClient endoflife.Client // Optional: for fetching lifecycle dates
	cacheTTL        time.Duration
	group           singleflight.Group // Prevents thundering herd on API calls
}

//nolint:govet // field alignment sacrificed for readability
type cachedEKSVersions struct {
	versions  []*types.VersionLifecycle
	fetchedAt time.Time
}

// NewEKSEOLProvider creates a new AWS EKS EOL provider
func NewEKSEOLProvider(client EKSClient, cacheTTL time.Duration) *EKSEOLProvider {
	if cacheTTL == 0 {
		cacheTTL = 24 * time.Hour // Default: cache for 24 hours
	}

	return &EKSEOLProvider{
		client:   client,
		cacheTTL: cacheTTL,
		cache:    make(map[string]*cachedEKSVersions),
	}
}

// WithEndOfLifeClient adds endoflife.date API integration for lifecycle date enrichment.
// When configured, the provider will attempt to fetch lifecycle dates from endoflife.date
// before falling back to static known dates.
func (p *EKSEOLProvider) WithEndOfLifeClient(client endoflife.Client) *EKSEOLProvider {
	p.endoflifeClient = client
	return p
}

// Name returns the name of this provider
func (p *EKSEOLProvider) Name() string {
	return "aws-eks-api"
}

// Engines returns the list of supported engines
func (p *EKSEOLProvider) Engines() []string {
	return []string{
		"kubernetes",
		"k8s",
		"eks",
	}
}

// GetVersionLifecycle retrieves lifecycle information for a specific version
func (p *EKSEOLProvider) GetVersionLifecycle(ctx context.Context, engine, version string) (*types.VersionLifecycle, error) {
	// Normalize engine name
	engine = strings.ToLower(engine)
	if !p.supportsEngine(engine) {
		return nil, fmt.Errorf("unsupported engine: %s", engine)
	}

	// Normalize version format (remove "k8s-" prefix if present)
	version = strings.TrimPrefix(version, "k8s-")
	version = strings.TrimPrefix(version, "kubernetes-")

	// Fetch all versions
	versions, err := p.ListAllVersions(ctx, engine)
	if err != nil {
		return nil, err
	}

	// Find the specific version
	for _, v := range versions {
		normalizedV := strings.TrimPrefix(v.Version, "k8s-")
		if normalizedV == version {
			return v, nil
		}
	}

	// Version not found - return unknown lifecycle (empty Version signals missing data)
	//
	// Design Decision: Return lifecycle with empty Version rather than error
	// Rationale:
	//   - Maintains observability: Resource tracked with UNKNOWN status vs lost entirely
	//   - Graceful degradation: Workflow continues during partial API outages
	//   - Policy decides: EOL provider fetches data, policy layer interprets "unknown"
	//
	// Alternative (rejected): Return error - would cause workflow to skip resource,
	// losing visibility into resources with incomplete EOL data coverage.
	return &types.VersionLifecycle{
		Version:     "", // Empty = unknown data, not unsupported version
		Engine:      engine,
		IsSupported: false,
		Source:      p.Name(),
		FetchedAt:   time.Now(),
	}, nil
}

// ListAllVersions retrieves all versions for kubernetes/eks
func (p *EKSEOLProvider) ListAllVersions(ctx context.Context, engine string) ([]*types.VersionLifecycle, error) {
	// Use "kubernetes" as the cache key for all engine variants
	cacheKey := "kubernetes"

	// Check cache first (fast path)
	p.mu.RLock()
	if cached, ok := p.cache[cacheKey]; ok {
		if time.Since(cached.fetchedAt) < p.cacheTTL {
			versions := cached.versions
			p.mu.RUnlock()
			return versions, nil
		}
	}
	p.mu.RUnlock()

	// Cache miss or expired - use singleflight to prevent thundering herd
	result, err, _ := p.group.Do(cacheKey, func() (interface{}, error) {
		// Fetch from AWS EKS API (only one goroutine executes this)
		awsVersions, err := p.client.DescribeAddonVersions(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to fetch EKS versions")
		}

		// Convert to our types
		var versions []*types.VersionLifecycle
		for _, av := range awsVersions {
			// Enrich with known lifecycle dates before conversion
			// Try endoflife.date first if configured, fall back to static dates
			enrichWithLifecycleDates(ctx, av, p.endoflifeClient)
			lifecycle := p.convertAWSVersion(av)
			versions = append(versions, lifecycle)
		}

		// Cache the result
		p.mu.Lock()
		p.cache[cacheKey] = &cachedEKSVersions{
			versions:  versions,
			fetchedAt: time.Now(),
		}
		p.mu.Unlock()

		return versions, nil
	})

	if err != nil {
		return nil, err
	}

	versions, ok := result.([]*types.VersionLifecycle)
	if !ok {
		return nil, errors.New("failed to convert result to VersionLifecycle slice")
	}
	return versions, nil
}

// convertAWSVersion converts an AWS EKSVersion to our VersionLifecycle type
func (p *EKSEOLProvider) convertAWSVersion(av *EKSVersion) *types.VersionLifecycle {
	version := av.KubernetesVersion
	// Normalize version to include k8s- prefix
	if !strings.HasPrefix(version, "k8s-") {
		version = "k8s-" + version
	}

	lifecycle := &types.VersionLifecycle{
		Version:   version,
		Engine:    "kubernetes",
		Source:    p.Name(),
		FetchedAt: time.Now(),
	}

	// Copy dates from EKSVersion (populated by enrichment)
	lifecycle.ReleaseDate = av.ReleaseDate
	lifecycle.DeprecationDate = av.EndOfStandardDate
	lifecycle.ExtendedSupportEnd = av.EndOfExtendedDate
	lifecycle.EOLDate = av.EndOfExtendedDate

	now := time.Now()
	status := strings.ToLower(av.Status)

	switch status {
	case eksStatusStandard:
		lifecycle.IsSupported = true
		lifecycle.IsDeprecated = false
		lifecycle.IsEOL = false

	case eksStatusExtended:
		lifecycle.IsSupported = true
		lifecycle.IsExtendedSupport = true
		lifecycle.IsDeprecated = true

	case eksStatusDeprecated:
		lifecycle.IsDeprecated = true
		lifecycle.IsSupported = false

		// Check if past end of extended support
		if av.EndOfExtendedDate != nil {
			if now.After(*av.EndOfExtendedDate) {
				lifecycle.IsEOL = true
				lifecycle.EOLDate = av.EndOfExtendedDate
			} else {
				// Still in extended support window
				lifecycle.IsExtendedSupport = true
			}
		} else {
			// No extended support info, consider EOL
			lifecycle.IsEOL = true
			if av.EndOfStandardDate != nil {
				lifecycle.EOLDate = av.EndOfStandardDate
			}
		}

	default:
		// Unknown status - conservative approach: mark as unsupported
		lifecycle.IsSupported = false
	}

	return lifecycle
}

// supportsEngine checks if the engine is supported by this provider
func (p *EKSEOLProvider) supportsEngine(engine string) bool {
	engine = strings.ToLower(engine)
	for _, e := range p.Engines() {
		if e == engine {
			return true
		}
	}
	return false
}

// enrichWithLifecycleDates adds known EOL dates for EKS versions
// Strategy: Try endoflife.date API first (if configured), fall back to static dates
func enrichWithLifecycleDates(ctx context.Context, version *EKSVersion, eolClient endoflife.Client) {
	// Only enrich if dates are missing
	if version.EndOfStandardDate != nil && version.EndOfExtendedDate != nil && version.ReleaseDate != nil {
		// Already has all dates, no need to enrich
		return
	}

	// Strategy 1: Try endoflife.date API (if configured)
	if eolClient != nil {
		if enrichFromEndOfLife(ctx, version, eolClient) {
			// Successfully enriched from endoflife.date
			updateStatusFromDates(version)
			return
		}
		// If endoflife.date fails, fall through to static dates
	}

	// Strategy 2: Fall back to static known dates from AWS documentation
	enrichWithStaticDates(version)
	updateStatusFromDates(version)
}

// enrichFromEndOfLife attempts to enrich version data from endoflife.date cycles
// Returns true if successful, false if data not found
//
// WARNING: Amazon EKS uses a NON-STANDARD schema on endoflife.date
//
// Standard endoflife.date semantics:
//   - cycle.EOL = true end of life date
//   - cycle.Support = end of standard support
//
// Amazon EKS DEVIATION (non-standard):
//   - cycle.EOL = end of STANDARD support (NOT true EOL!)
//   - cycle.ExtendedSupport = end of EXTENDED support (true EOL)
//   - cycle.Support = often empty/missing
//
// This is why EKS MUST use EKSEOLProvider with this custom field mapping
// instead of the generic endoflife.Provider (which would interpret dates incorrectly).
func enrichFromEndOfLife(ctx context.Context, version *EKSVersion, client endoflife.Client) bool {
	cycles, err := client.GetProductCycles(ctx, "amazon-eks")
	if err != nil {
		// API error - fall back to static dates
		return false
	}

	// Find matching cycle
	for _, cycle := range cycles {
		if cycle.Cycle != version.KubernetesVersion {
			continue
		}

		// Found matching cycle - extract dates
		if version.ReleaseDate == nil && cycle.ReleaseDate != "" {
			if releaseDate, err := time.Parse("2006-01-02", cycle.ReleaseDate); err == nil {
				version.ReleaseDate = &releaseDate
			}
		}

		// End of standard support from EOL field (EKS NON-STANDARD mapping!)
		if version.EndOfStandardDate == nil && cycle.EOL != "" && cycle.EOL != "false" {
			if eolDate, err := time.Parse("2006-01-02", cycle.EOL); err == nil {
				version.EndOfStandardDate = &eolDate
			}
		}

		// End of extended support from extendedSupport field
		if version.EndOfExtendedDate == nil {
			if cycle.ExtendedSupport != nil {
				switch v := cycle.ExtendedSupport.(type) {
				case string:
					if v != "" && v != "false" {
						if extDate, err := time.Parse("2006-01-02", v); err == nil {
							version.EndOfExtendedDate = &extDate
						}
					}
				}
			}
		}

		return true
	}

	// Version not found in endoflife.date
	return false
}

// enrichWithStaticDates enriches version data with static known dates from AWS documentation
// Source: https://docs.aws.amazon.com/eks/latest/userguide/kubernetes-versions.html
func enrichWithStaticDates(version *EKSVersion) {
	knownDates := map[string]struct {
		releaseDate   string
		endOfStandard string
		endOfExtended string
	}{
		"1.31": {"2024-11-19", "2025-12-19", "2027-05-19"},
		"1.30": {"2024-05-29", "2025-06-29", "2026-11-29"},
		"1.29": {"2024-01-23", "2025-02-23", "2026-07-23"},
		"1.28": {"2023-09-26", "2024-10-26", "2026-03-26"},
		"1.27": {"2023-05-24", "2024-06-24", "2025-11-24"},
		"1.26": {"2023-04-11", "2024-05-11", "2025-10-11"},
		"1.25": {"2023-02-21", "2024-03-21", "2025-08-21"},
		"1.24": {"2022-11-15", "2023-12-15", "2025-05-15"},
		"1.23": {"2022-08-11", "2023-09-11", "2025-02-11"},
	}

	dates, exists := knownDates[version.KubernetesVersion]
	if !exists {
		return
	}

	if version.ReleaseDate == nil {
		if releaseDate, err := time.Parse("2006-01-02", dates.releaseDate); err == nil {
			version.ReleaseDate = &releaseDate
		}
	}
	if version.EndOfStandardDate == nil {
		if endStandard, err := time.Parse("2006-01-02", dates.endOfStandard); err == nil {
			version.EndOfStandardDate = &endStandard
		}
	}
	if version.EndOfExtendedDate == nil {
		if endExtended, err := time.Parse("2006-01-02", dates.endOfExtended); err == nil {
			version.EndOfExtendedDate = &endExtended
		}
	}
}

// updateStatusFromDates updates version status based on lifecycle dates
func updateStatusFromDates(version *EKSVersion) {
	// Update status based on dates if not already set
	if version.Status == "" || version.Status == eksStatusExtended || version.Status == eksStatusStandard {
		now := time.Now()
		if version.EndOfExtendedDate != nil && now.After(*version.EndOfExtendedDate) {
			version.Status = eksStatusDeprecated
		} else if version.EndOfStandardDate != nil && now.After(*version.EndOfStandardDate) {
			version.Status = eksStatusExtended
		} else if version.Status == "" {
			version.Status = eksStatusStandard
		}
	}
}
