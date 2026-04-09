package aws

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/sync/singleflight"

	"github.com/block/Version-Guard/pkg/types"
)

// RDSClient defines the interface for AWS RDS API calls
// This allows us to mock the AWS SDK for testing
type RDSClient interface {
	// DescribeDBEngineVersions retrieves engine version information from AWS RDS
	DescribeDBEngineVersions(ctx context.Context, engine string) ([]*EngineVersion, error)
}

//nolint:govet // field alignment sacrificed for readability
type EngineVersion struct {
	Engine                  string
	EngineVersion           string
	Status                  string // "available", "deprecated"
	SupportedEngineModes    []string
	ValidUpgradeTarget      []string
	DeprecationDate         *time.Time
	SupportsClusters        bool
	SupportsGlobalDatabases bool
}

// RDSEOLProvider fetches EOL data from AWS RDS DescribeDBEngineVersions API
//
//nolint:govet // field alignment sacrificed for readability
type RDSEOLProvider struct {
	mu       sync.RWMutex
	cache    map[string]*cachedVersions
	client   RDSClient
	cacheTTL time.Duration
	group    singleflight.Group // Prevents thundering herd on API calls
}

//nolint:govet // field alignment sacrificed for readability
type cachedVersions struct {
	versions  []*types.VersionLifecycle
	fetchedAt time.Time
}

// NewRDSEOLProvider creates a new AWS RDS EOL provider
func NewRDSEOLProvider(client RDSClient, cacheTTL time.Duration) *RDSEOLProvider {
	if cacheTTL == 0 {
		cacheTTL = 24 * time.Hour // Default: cache for 24 hours
	}

	return &RDSEOLProvider{
		client:   client,
		cacheTTL: cacheTTL,
		cache:    make(map[string]*cachedVersions),
	}
}

// Name returns the name of this provider
func (p *RDSEOLProvider) Name() string {
	return "aws-rds-api"
}

// Engines returns the list of supported engines
func (p *RDSEOLProvider) Engines() []string {
	return []string{
		"aurora-mysql",
		"aurora-postgresql",
		"mysql",
		"postgres",
		"mariadb",
	}
}

// GetVersionLifecycle retrieves lifecycle information for a specific version
func (p *RDSEOLProvider) GetVersionLifecycle(ctx context.Context, engine, version string) (*types.VersionLifecycle, error) {
	// Fetch all versions for this engine
	versions, err := p.ListAllVersions(ctx, engine)
	if err != nil {
		return nil, err
	}

	// Find the specific version
	for _, v := range versions {
		if v.Version == version {
			return v, nil
		}
	}

	// Version not found - return unknown lifecycle (empty Version signals missing data)
	// Policy will classify as UNKNOWN (data gap) rather than RED/YELLOW (user issue)
	return &types.VersionLifecycle{
		Version:     "", // Empty = unknown data, not unsupported version
		Engine:      engine,
		IsSupported: false,
		Source:      p.Name(),
		FetchedAt:   time.Now(),
	}, nil
}

// ListAllVersions retrieves all versions for an engine
func (p *RDSEOLProvider) ListAllVersions(ctx context.Context, engine string) ([]*types.VersionLifecycle, error) {
	// Check cache first (fast path)
	p.mu.RLock()
	if cached, ok := p.cache[engine]; ok {
		if time.Since(cached.fetchedAt) < p.cacheTTL {
			versions := cached.versions
			p.mu.RUnlock()
			return versions, nil
		}
	}
	p.mu.RUnlock()

	// Cache miss or expired - use singleflight to prevent thundering herd
	result, err, _ := p.group.Do(engine, func() (interface{}, error) {
		// Fetch from AWS RDS API (only one goroutine executes this)
		awsVersions, err := p.client.DescribeDBEngineVersions(ctx, engine)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to fetch engine versions for %s", engine)
		}

		// Convert to our types
		var versions []*types.VersionLifecycle
		for _, av := range awsVersions {
			lifecycle := p.convertAWSVersion(av)
			versions = append(versions, lifecycle)
		}

		// Cache the result
		p.mu.Lock()
		p.cache[engine] = &cachedVersions{
			versions:  versions,
			fetchedAt: time.Now(),
		}
		p.mu.Unlock()

		return versions, nil
	})

	if err != nil {
		return nil, err
	}

	return result.([]*types.VersionLifecycle), nil
}

// convertAWSVersion converts an AWS EngineVersion to our VersionLifecycle type
func (p *RDSEOLProvider) convertAWSVersion(av *EngineVersion) *types.VersionLifecycle {
	lifecycle := &types.VersionLifecycle{
		Version:   av.EngineVersion,
		Engine:    av.Engine,
		Source:    p.Name(),
		FetchedAt: time.Now(),
	}

	// Determine status based on AWS fields
	status := strings.ToLower(av.Status)

	switch status {
	case "deprecated":
		lifecycle.IsDeprecated = true
		lifecycle.IsSupported = false
		lifecycle.DeprecationDate = av.DeprecationDate

	case "available":
		lifecycle.IsSupported = true
		lifecycle.IsDeprecated = false

	default:
		// Unknown status - conservative approach: mark as unsupported
		lifecycle.IsSupported = false
	}

	// Check if in extended support
	// For AWS RDS, extended support is typically indicated by:
	// 1. Deprecated status but still available
	// 2. Has a deprecation date in the past but status is still "available"
	if av.DeprecationDate != nil && time.Now().After(*av.DeprecationDate) && status == "available" {
		lifecycle.IsExtendedSupport = true
		// Extended support typically lasts 1-3 years after deprecation
		// For Aurora MySQL 5.6, for example, extended support ended in Nov 2024
		extendedSupportEnd := av.DeprecationDate.AddDate(1, 0, 0) // +1 year
		lifecycle.ExtendedSupportEnd = &extendedSupportEnd
	}

	// Determine if EOL (past extended support or marked as deprecated with no upgrade path)
	if lifecycle.ExtendedSupportEnd != nil && time.Now().After(*lifecycle.ExtendedSupportEnd) {
		lifecycle.IsEOL = true
		lifecycle.EOLDate = lifecycle.ExtendedSupportEnd
	} else if lifecycle.IsDeprecated && len(av.ValidUpgradeTarget) == 0 {
		// Deprecated with no upgrade path = effectively EOL
		lifecycle.IsEOL = true
		if av.DeprecationDate != nil {
			lifecycle.EOLDate = av.DeprecationDate
		}
	}

	return lifecycle
}
