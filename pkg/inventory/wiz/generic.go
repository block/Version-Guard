package wiz

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/block/Version-Guard/pkg/config"
	"github.com/block/Version-Guard/pkg/registry"
	"github.com/block/Version-Guard/pkg/types"
)

const (
	resourceTypeEKS        = "eks"
	resourceTypeOpenSearch = "opensearch"
	resourceTypeLambda     = "lambda"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const discoveredAtKey contextKey = "discovered_at"

// GenericInventorySource is a config-driven Wiz inventory source
// that can handle any resource type based on YAML configuration
type GenericInventorySource struct {
	client         *Client
	config         *config.ResourceConfig
	registryClient registry.Client
	logger         *slog.Logger
}

// NewGenericInventorySource creates a new generic inventory source from config
func NewGenericInventorySource(
	client *Client,
	cfg *config.ResourceConfig,
	registryClient registry.Client,
	logger *slog.Logger,
) *GenericInventorySource {
	if logger == nil {
		logger = slog.Default()
	}
	return &GenericInventorySource{
		client:         client,
		config:         cfg,
		registryClient: registryClient,
		logger:         logger,
	}
}

// Name returns the name of this inventory source
func (s *GenericInventorySource) Name() string {
	return "wiz-" + s.config.ID
}

// CloudProvider returns the cloud provider for this source
func (s *GenericInventorySource) CloudProvider() types.CloudProvider {
	switch s.config.CloudProvider {
	case "aws":
		return types.CloudProviderAWS
	case "gcp":
		return types.CloudProviderGCP
	case "azure":
		return types.CloudProviderAzure
	default:
		return types.CloudProviderAWS // Default to AWS
	}
}

// ListResources fetches resources from Wiz using the configured report.
// The resourceType parameter is accepted for interface compatibility but
// the source uses its own config type internally.
func (s *GenericInventorySource) ListResources(ctx context.Context, resourceType types.ResourceType) ([]*types.Resource, error) {
	// Get report ID from WIZ_REPORT_IDS map
	reportID, err := getReportIDFromMap(s.config.ID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get report ID for resource %s", s.config.ID)
	}
	if reportID == "" {
		return nil, errors.Errorf("no report ID configured for resource %s in WIZ_REPORT_IDS map", s.config.ID)
	}

	// Determine required columns from field mappings
	requiredColumns := s.getRequiredColumns()

	// Filter function: check nativeType pattern
	filterRow := func(cols columnIndex, row []string) bool {
		nativeType := cols.col(row, colHeaderNativeType)
		return s.matchesNativeTypePattern(nativeType)
	}

	// Parser function: parse row into Resource
	parseRow := func(ctx context.Context, cols columnIndex, row []string) (*types.Resource, error) {
		return s.parseResourceRow(ctx, cols, row)
	}

	// Use shared helper to parse Wiz report
	return parseWizReport(
		ctx,
		s.client,
		reportID,
		requiredColumns,
		filterRow,
		parseRow,
		s.logger,
	)
}

// GetResource fetches a single resource by ID
func (s *GenericInventorySource) GetResource(ctx context.Context, resourceType types.ResourceType, id string) (*types.Resource, error) {
	resources, err := s.ListResources(ctx, resourceType)
	if err != nil {
		return nil, err
	}

	for _, r := range resources {
		if r.ID == id {
			return r, nil
		}
	}

	return nil, errors.Errorf("resource not found: %s", id)
}

// wellKnownFieldMappingKeys are the YAML field_mappings keys whose
// values flow into typed fields on Resource. The set is intentionally
// minimal: only the values the system itself depends on (identity,
// version-lookup keys, and tags for service derivation) are typed.
// Everything else — name, account_id, region, owner, cost_center, ... —
// is routed verbatim into Resource.Extra under its YAML logical name,
// so adding new per-resource attributes is a YAML-only change.
var wellKnownFieldMappingKeys = map[string]struct{}{
	"resource_id": {},
	"version":     {},
	"engine":      {},
	"tags":        {},
}

// column returns the CSV column to use for the given field-mapping key,
// falling back to defaultCol when the resource config does not override
// the mapping. This makes every well-known field driven by YAML rather
// than hard-coded constants, so a new resource type can use a different
// Wiz column without code changes (e.g. EKS using "providerUniqueId"
// instead of "externalId").
func (s *GenericInventorySource) column(key, defaultCol string) string {
	if mapped, ok := s.config.Inventory.FieldMappings[key]; ok && mapped != "" {
		return mapped
	}
	return defaultCol
}

// getRequiredColumns builds the list of CSV columns the parser will read,
// derived entirely from field_mappings (with Wiz defaults for the typed
// keys: resource_id and tags).
//
// The "nativeType" column is always required because it is used to filter
// rows down to the resource type before any field extraction happens.
func (s *GenericInventorySource) getRequiredColumns() []string {
	columns := []string{
		s.column("resource_id", colHeaderExternalID),
		colHeaderNativeType,
		s.column("tags", colHeaderTags),
	}

	// Add version if mapped
	if v := s.column("version", ""); v != "" {
		columns = append(columns, v)
	}

	// Add engine if mapped
	if e := s.column("engine", ""); e != "" {
		columns = append(columns, e)
	}

	// Every non-typed YAML field_mappings entry (name, account_id,
	// region, owner, cost_center, ...) is required so the Wiz header
	// validator catches typos in YAML at parse start instead of
	// silently producing empty Extra values.
	for key, col := range s.config.Inventory.FieldMappings {
		if _, isWellKnown := wellKnownFieldMappingKeys[key]; isWellKnown {
			continue
		}
		if col != "" {
			columns = append(columns, col)
		}
	}

	// Lambda needs graphEntity.properties for runtime extraction
	if s.config.Type == resourceTypeLambda {
		columns = append(columns, colHeaderGraphProperties)
	}

	return columns
}

// matchesNativeTypePattern checks if nativeType matches the configured pattern.
// Supports exact match, wildcard patterns (e.g., "elastiCache/*/cluster"),
// and pipe-delimited alternatives (e.g., "elasticSearchService|OpenSearch Domain").
func (s *GenericInventorySource) matchesNativeTypePattern(nativeType string) bool {
	pattern := s.config.Inventory.NativeTypePattern

	// Handle pipe-delimited alternatives (e.g., "typeA|typeB")
	if strings.Contains(pattern, "|") {
		for _, alt := range strings.Split(pattern, "|") {
			if nativeType == alt {
				return true
			}
		}
		return false
	}

	// Handle wildcard patterns (e.g., "elastiCache/*/cluster")
	if strings.Contains(pattern, "*") {
		parts := strings.Split(pattern, "/")
		typeParts := strings.Split(nativeType, "/")

		if len(parts) != len(typeParts) {
			return false
		}

		for i, part := range parts {
			if part != "*" && part != typeParts[i] {
				return false
			}
		}
		return true
	}

	// Exact match
	return nativeType == pattern
}

// parseResourceRow parses a CSV row into a Resource using field mappings.
//
// Only typed keys (resource_id, version, engine, tags) are read directly
// onto the Resource. Every other field_mappings entry — including name,
// account_id, region — is routed verbatim into Resource.Extra under its
// YAML logical name. resource_id is the only mapping we strictly require
// to be present in the row; missing values for everything else just
// produce empty strings.
func (s *GenericInventorySource) parseResourceRow(
	ctx context.Context,
	cols columnIndex,
	row []string,
) (*types.Resource, error) {
	resourceID, err := cols.require(row, s.column("resource_id", colHeaderExternalID))
	if err != nil {
		return nil, err
	}

	version := cols.col(row, s.column("version", ""))
	engine := cols.col(row, s.column("engine", ""))

	// Lambda-specific: extract runtime from graphEntity.properties JSON.
	// The runtime string (e.g., "python3.12", "nodejs20.x") is the cycle
	// identifier on endoflife.date's aws-lambda product, so it becomes the
	// version. The engine is always "aws-lambda".
	//
	// Container-image Lambdas have runtime=null in Wiz because the runtime
	// is baked into the Docker image, not managed by AWS. Since AWS doesn't
	// EOL container-image Lambdas (there's no runtime deprecation date),
	// they're out of scope — skip them to avoid noise in findings.
	if s.config.Type == resourceTypeLambda {
		propsJSON := cols.col(row, colHeaderGraphProperties)
		runtime := extractLambdaRuntime(propsJSON)
		if runtime == "" {
			return nil, nil
		}
		version = runtime
		engine = "aws-lambda"
	}

	// For EKS, default to "eks" if no engine field is mapped
	if s.config.Type == resourceTypeEKS && engine == "" {
		engine = resourceTypeEKS
	}

	// Normalize engine
	engine = normalizeEngine(engine, s.config.Type)

	// OpenSearch-specific: normalize version and detect legacy Elasticsearch
	if s.config.Type == resourceTypeOpenSearch {
		version = normalizeOpenSearchVersion(version)
		engine = detectOpenSearchEngine(version)
	}

	// Parse tags to extract service
	tagsJSON := cols.col(row, s.column("tags", colHeaderTags))
	tags, err := ParseTags(tagsJSON)
	if err != nil {
		s.logger.WarnContext(ctx, "failed to parse tags",
			"resource_id", resourceID,
			"error", err)
		tags = nil
	}

	// Collect every non-typed YAML field_mappings value into Extra,
	// keyed by the YAML logical name. Empty strings still produce an
	// entry so downstream code can distinguish "configured but missing"
	// from "not configured at all".
	var extra map[string]string
	for key, col := range s.config.Inventory.FieldMappings {
		if _, isWellKnown := wellKnownFieldMappingKeys[key]; isWellKnown {
			continue
		}
		if col == "" {
			continue
		}
		if extra == nil {
			extra = make(map[string]string)
		}
		extra[key] = cols.col(row, col)
	}

	// Service derivation: prefer the configured app tag; if none is
	// present, fall back to extracting a service name from Extra["name"]
	// (which is the cloud resource name when the user has mapped it).
	tagConfig := DefaultTagConfig()
	service := tagConfig.GetAppTag(tags)
	if service == "" {
		service = extractServiceFromName(extra["name"])
	}

	// Build resource
	discoveredAt := time.Now()
	if ctxTime, ok := ctx.Value(discoveredAtKey).(time.Time); ok {
		discoveredAt = ctxTime
	}

	resource := &types.Resource{
		ID:             resourceID,
		Type:           types.ResourceType(s.config.ID),
		CloudProvider:  s.CloudProvider(),
		CurrentVersion: version,
		Engine:         engine,
		Service:        service,
		Tags:           tags,
		Extra:          extra,
		DiscoveredAt:   discoveredAt,
	}

	return resource, nil
}

// getReportIDFromMap reads the WIZ_REPORT_IDS JSON map and returns the report ID for the given resource
func getReportIDFromMap(resourceID string) (string, error) {
	// Read WIZ_REPORT_IDS environment variable
	reportIDsJSON := os.Getenv("WIZ_REPORT_IDS")
	if reportIDsJSON == "" {
		return "", errors.New("WIZ_REPORT_IDS environment variable not set")
	}

	// Parse JSON map
	var reportIDs map[string]string
	if err := json.Unmarshal([]byte(reportIDsJSON), &reportIDs); err != nil {
		return "", errors.Wrap(err, "failed to parse WIZ_REPORT_IDS JSON")
	}

	// Get report ID for this resource
	reportID, ok := reportIDs[resourceID]
	if !ok {
		return "", nil // Not found in map, but not an error - let caller decide
	}

	return reportID, nil
}

// normalizeOpenSearchVersion strips engine prefixes from OpenSearch/Elasticsearch
// version strings (e.g., "OpenSearch_2.13" → "2.13", "Elasticsearch_7.10" → "7.10").
func normalizeOpenSearchVersion(version string) string {
	version = strings.TrimPrefix(version, "OpenSearch_")
	version = strings.TrimPrefix(version, "Elasticsearch_")
	return version
}

// detectOpenSearchEngine returns "elasticsearch" for legacy Elasticsearch versions
// (5.x, 6.x, 7.x) and "opensearch" for OpenSearch versions (1.x, 2.x, 3.x+).
// OpenSearch forked from Elasticsearch 7.10, so versions ≤7.x are Elasticsearch.
func detectOpenSearchEngine(version string) string {
	if version == "" {
		return resourceTypeOpenSearch
	}
	major := strings.SplitN(version, ".", 2)[0]
	switch major {
	case "5", "6", "7":
		return "elasticsearch"
	default:
		return resourceTypeOpenSearch
	}
}

// extractLambdaRuntime extracts the runtime identifier from the
// graphEntity.properties JSON column of a Wiz Lambda report row.
// The JSON contains a "runtime" field with values like "python3.12",
// "nodejs20.x", "java21", "provided.al2023", etc.
// Returns "" if the JSON is empty, unparseable, or has no runtime field.
func extractLambdaRuntime(propsJSON string) string {
	propsJSON = strings.TrimSpace(propsJSON)
	if propsJSON == "" {
		return ""
	}

	var props map[string]interface{}
	if err := json.Unmarshal([]byte(propsJSON), &props); err != nil {
		return ""
	}

	runtime, ok := props["runtime"].(string)
	if !ok {
		return ""
	}

	return strings.TrimSpace(runtime)
}

// normalizeEngine normalizes engine names based on resource type
func normalizeEngine(engine, resourceType string) string {
	engine = strings.ToLower(strings.TrimSpace(engine))

	// Handle type-specific normalization
	switch resourceType {
	case "aurora":
		// AuroraMySQL → aurora-mysql
		// AuroraPostgreSQL → aurora-postgresql
		if strings.Contains(engine, "aurora") {
			if strings.Contains(engine, "mysql") {
				return "aurora-mysql"
			}
			if strings.Contains(engine, "postgres") {
				return "aurora-postgresql"
			}
		}
	case "elasticache":
		// Redis → redis, Valkey → valkey, Memcached → memcached
		return engine
	case resourceTypeEKS:
		// Kubernetes → eks
		if strings.Contains(engine, "k8s") || strings.Contains(engine, "kubernetes") {
			return resourceTypeEKS
		}
	}

	return engine
}
