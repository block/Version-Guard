package wiz

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/block/Version-Guard/pkg/config"
	"github.com/block/Version-Guard/pkg/types"
)

func TestNewGenericInventorySource(t *testing.T) {
	client := &Client{}
	cfg := config.ResourceConfig{
		ID:            "test-resource",
		Type:          "aurora",
		CloudProvider: "aws",
	}

	source := NewGenericInventorySource(client, &cfg, nil, nil)

	assert.NotNil(t, source)
	assert.Equal(t, "wiz-test-resource", source.Name())
	assert.Equal(t, types.CloudProviderAWS, source.CloudProvider())
}

func TestGenericInventorySource_CloudProvider(t *testing.T) {
	tests := []struct {
		name           string
		cloudProvider  string
		expectedResult types.CloudProvider
	}{
		{
			name:           "AWS",
			cloudProvider:  "aws",
			expectedResult: types.CloudProviderAWS,
		},
		{
			name:           "GCP",
			cloudProvider:  "gcp",
			expectedResult: types.CloudProviderGCP,
		},
		{
			name:           "Azure",
			cloudProvider:  "azure",
			expectedResult: types.CloudProviderAzure,
		},
		{
			name:           "Unknown defaults to AWS",
			cloudProvider:  "unknown",
			expectedResult: types.CloudProviderAWS,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.ResourceConfig{
				CloudProvider: tt.cloudProvider,
			}
			source := NewGenericInventorySource(&Client{}, &cfg, nil, nil)
			assert.Equal(t, tt.expectedResult, source.CloudProvider())
		})
	}
}

func TestGetReportIDFromMap(t *testing.T) {
	tests := []struct {
		name         string
		envValue     string
		resourceID   string
		expectedID   string
		errorMessage string
		expectError  bool
	}{
		{
			name:        "Valid JSON with matching resource ID",
			envValue:    `{"aurora-postgresql":"report-123","eks":"report-456"}`,
			resourceID:  "aurora-postgresql",
			expectedID:  "report-123",
			expectError: false,
		},
		{
			name:        "Valid JSON with different resource ID",
			envValue:    `{"aurora-postgresql":"report-123","eks":"report-456"}`,
			resourceID:  "eks",
			expectedID:  "report-456",
			expectError: false,
		},
		{
			name:        "Resource ID not in map",
			envValue:    `{"aurora-postgresql":"report-123"}`,
			resourceID:  "eks",
			expectedID:  "",
			expectError: false,
		},
		{
			name:         "WIZ_REPORT_IDS not set",
			envValue:     "",
			resourceID:   "aurora-postgresql",
			expectedID:   "",
			expectError:  true,
			errorMessage: "WIZ_REPORT_IDS environment variable not set",
		},
		{
			name:         "Invalid JSON",
			envValue:     `{invalid json}`,
			resourceID:   "aurora-postgresql",
			expectedID:   "",
			expectError:  true,
			errorMessage: "failed to parse WIZ_REPORT_IDS JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment
			if tt.envValue != "" {
				os.Setenv("WIZ_REPORT_IDS", tt.envValue)
				defer os.Unsetenv("WIZ_REPORT_IDS")
			} else {
				os.Unsetenv("WIZ_REPORT_IDS")
			}

			reportID, err := getReportIDFromMap(tt.resourceID)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMessage)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedID, reportID)
			}
		})
	}
}

func TestMatchesNativeTypePattern(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		nativeType  string
		shouldMatch bool
	}{
		{
			name:        "Exact match",
			pattern:     "rds/AmazonAuroraPostgreSQL/cluster",
			nativeType:  "rds/AmazonAuroraPostgreSQL/cluster",
			shouldMatch: true,
		},
		{
			name:        "No match - different type",
			pattern:     "rds/AmazonAuroraPostgreSQL/cluster",
			nativeType:  "rds/AmazonAuroraMySQL/cluster",
			shouldMatch: false,
		},
		{
			name:        "Wildcard match - middle segment",
			pattern:     "elastiCache/*/cluster",
			nativeType:  "elastiCache/Redis/cluster",
			shouldMatch: true,
		},
		{
			name:        "Wildcard match - different value",
			pattern:     "elastiCache/*/cluster",
			nativeType:  "elastiCache/Valkey/cluster",
			shouldMatch: true,
		},
		{
			name:        "Wildcard no match - wrong prefix",
			pattern:     "elastiCache/*/cluster",
			nativeType:  "rds/Redis/cluster",
			shouldMatch: false,
		},
		{
			name:        "Wildcard no match - wrong suffix",
			pattern:     "elastiCache/*/cluster",
			nativeType:  "elastiCache/Redis/instance",
			shouldMatch: false,
		},
		{
			name:        "Wildcard no match - different segment count",
			pattern:     "elastiCache/*/cluster",
			nativeType:  "elastiCache/Redis/cluster/extra",
			shouldMatch: false,
		},
		{
			name:        "EKS exact match",
			pattern:     "eks/Cluster",
			nativeType:  "eks/Cluster",
			shouldMatch: true,
		},
		{
			name:        "Lambda exact match",
			pattern:     "lambda",
			nativeType:  "lambda",
			shouldMatch: true,
		},
		{
			name:        "Lambda no match - different type",
			pattern:     "lambda",
			nativeType:  "lambda/Python/function",
			shouldMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.ResourceConfig{
				Inventory: config.InventoryConfig{
					NativeTypePattern: tt.pattern,
				},
			}
			source := NewGenericInventorySource(&Client{}, &cfg, nil, nil)
			result := source.matchesNativeTypePattern(tt.nativeType)
			assert.Equal(t, tt.shouldMatch, result)
		})
	}
}

func TestNormalizeEngine(t *testing.T) {
	tests := []struct {
		name         string
		engine       string
		resourceType string
		expected     string
	}{
		{
			name:         "Aurora MySQL",
			engine:       "AuroraMySQL",
			resourceType: "aurora",
			expected:     "aurora-mysql",
		},
		{
			name:         "Aurora PostgreSQL",
			engine:       "AuroraPostgreSQL",
			resourceType: "aurora",
			expected:     "aurora-postgresql",
		},
		{
			name:         "Aurora MySQL lowercase",
			engine:       "auroramysql",
			resourceType: "aurora",
			expected:     "aurora-mysql",
		},
		{
			name:         "Redis",
			engine:       "Redis",
			resourceType: "elasticache",
			expected:     "redis",
		},
		{
			name:         "Valkey",
			engine:       "Valkey",
			resourceType: "elasticache",
			expected:     "valkey",
		},
		{
			name:         "EKS with kubernetes",
			engine:       "Kubernetes",
			resourceType: "eks",
			expected:     "eks",
		},
		{
			name:         "EKS with k8s",
			engine:       "k8s",
			resourceType: "eks",
			expected:     "eks",
		},
		{
			name:         "Trim whitespace",
			engine:       "  Redis  ",
			resourceType: "elasticache",
			expected:     "redis",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeEngine(tt.engine, tt.resourceType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseResourceRow(t *testing.T) {
	cfg := config.ResourceConfig{
		ID:            "aurora-postgresql",
		Type:          "aurora",
		CloudProvider: "aws",
		Inventory: config.InventoryConfig{
			FieldMappings: map[string]string{
				"version": "versionDetails.version",
				"engine":  "typeFields.kind",
			},
		},
	}

	source := NewGenericInventorySource(&Client{}, &cfg, nil, nil)

	cols := columnIndex{
		colHeaderExternalID:      0,
		colHeaderName:            1,
		colHeaderAccountID:       2,
		colHeaderRegion:          3,
		"versionDetails.version": 4,
		"typeFields.kind":        5,
		colHeaderTags:            6,
	}

	tagsJSON := `[{"key":"app","value":"my-service"},{"key":"brand","value":"afterpay"}]`

	row := []string{
		"arn:aws:rds:us-west-2:123456789012:cluster:my-cluster", // external_id
		"my-cluster",       // name
		"123456789012",     // account_id
		"us-west-2",        // region
		"15.3",             // version
		"AuroraPostgreSQL", // engine
		tagsJSON,           // tags
	}

	ctx := context.Background()
	resource, err := source.parseResourceRow(ctx, cols, row)

	require.NoError(t, err)
	assert.Equal(t, "arn:aws:rds:us-west-2:123456789012:cluster:my-cluster", resource.ID)
	assert.Equal(t, "my-cluster", resource.Name)
	assert.Equal(t, types.ResourceType("aurora-postgresql"), resource.Type)
	assert.Equal(t, types.CloudProviderAWS, resource.CloudProvider)
	assert.Equal(t, "123456789012", resource.CloudAccountID)
	assert.Equal(t, "us-west-2", resource.CloudRegion)
	assert.Equal(t, "15.3", resource.CurrentVersion)
	assert.Equal(t, "aurora-postgresql", resource.Engine)
	assert.Equal(t, "my-service", resource.Service)
	assert.Equal(t, "afterpay", resource.Brand)
	// Verify all tags are stored
	assert.NotNil(t, resource.Tags)
	assert.Equal(t, "my-service", resource.Tags["app"])
	assert.Equal(t, "afterpay", resource.Tags["brand"])
}

func TestParseResourceRow_MissingRequiredFields(t *testing.T) {
	cfg := config.ResourceConfig{
		ID:            "test",
		Type:          "aurora",
		CloudProvider: "aws",
	}

	source := NewGenericInventorySource(&Client{}, &cfg, nil, nil)

	cols := columnIndex{
		colHeaderName: 0,
	}

	row := []string{"test-name"}

	ctx := context.Background()
	_, err := source.parseResourceRow(ctx, cols, row)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "externalId")
}

func TestParseResourceRow_FallbackToExternalIDForName(t *testing.T) {
	cfg := config.ResourceConfig{
		ID:            "test",
		Type:          "aurora",
		CloudProvider: "aws",
	}

	source := NewGenericInventorySource(&Client{}, &cfg, nil, nil)

	cols := columnIndex{
		colHeaderExternalID: 0,
		colHeaderName:       1,
		colHeaderAccountID:  2,
	}

	row := []string{
		"test-external-id",
		"", // Empty name
		"123456789012",
	}

	ctx := context.Background()
	resource, err := source.parseResourceRow(ctx, cols, row)

	require.NoError(t, err)
	assert.Equal(t, "test-external-id", resource.Name, "should fallback to external ID when name is empty")
}

func TestGetRequiredColumns(t *testing.T) {
	cfg := config.ResourceConfig{
		Inventory: config.InventoryConfig{
			FieldMappings: map[string]string{
				"version": "versionDetails.version",
				"engine":  "typeFields.kind",
			},
		},
	}

	source := NewGenericInventorySource(&Client{}, &cfg, nil, nil)
	columns := source.getRequiredColumns()

	// Check base columns
	assert.Contains(t, columns, colHeaderExternalID)
	assert.Contains(t, columns, colHeaderName)
	assert.Contains(t, columns, colHeaderNativeType)
	assert.Contains(t, columns, colHeaderAccountID)
	assert.Contains(t, columns, colHeaderRegion)
	assert.Contains(t, columns, colHeaderTags)

	// Check mapped columns
	assert.Contains(t, columns, "versionDetails.version")
	assert.Contains(t, columns, "typeFields.kind")
}

func TestGetRequiredColumns_NoMappings(t *testing.T) {
	cfg := config.ResourceConfig{
		Inventory: config.InventoryConfig{
			FieldMappings: map[string]string{},
		},
	}

	source := NewGenericInventorySource(&Client{}, &cfg, nil, nil)
	columns := source.getRequiredColumns()

	// Should still have base columns
	assert.Contains(t, columns, colHeaderExternalID)
	assert.Contains(t, columns, colHeaderName)
	assert.Len(t, columns, 6) // Only the 6 base columns
}

func TestListResources_NoReportID(t *testing.T) {
	// Ensure WIZ_REPORT_IDS is not set
	os.Unsetenv("WIZ_REPORT_IDS")

	cfg := config.ResourceConfig{
		ID:   "aurora-postgresql",
		Type: "aurora",
	}

	source := NewGenericInventorySource(&Client{}, &cfg, nil, nil)

	ctx := context.Background()
	_, err := source.ListResources(ctx, types.ResourceType("aurora"))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get report ID")
}

func TestListResources_ReportIDNotInMap(t *testing.T) {
	reportIDs := map[string]string{
		"eks": "report-456",
	}
	reportIDsJSON, _ := json.Marshal(reportIDs)
	os.Setenv("WIZ_REPORT_IDS", string(reportIDsJSON))
	defer os.Unsetenv("WIZ_REPORT_IDS")

	cfg := config.ResourceConfig{
		ID:   "aurora-postgresql",
		Type: "aurora",
	}

	source := NewGenericInventorySource(&Client{}, &cfg, nil, nil)

	ctx := context.Background()
	_, err := source.ListResources(ctx, types.ResourceType("aurora"))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no report ID configured for resource aurora-postgresql")
}

func TestGetResource(t *testing.T) {
	// Note: This test would require mocking the Wiz client
	// For now, we test the error case when ListResources fails

	os.Unsetenv("WIZ_REPORT_IDS")

	cfg := config.ResourceConfig{
		ID:   "test",
		Type: "aurora",
	}

	source := NewGenericInventorySource(&Client{}, &cfg, nil, nil)

	ctx := context.Background()
	_, err := source.GetResource(ctx, types.ResourceType("aurora"), "test-id")

	assert.Error(t, err)
}

func TestExtractLambdaRuntime(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		expected string
	}{
		{
			name:     "Python runtime",
			json:     `{"runtime":"python3.12","memorySize":256}`,
			expected: "python3.12",
		},
		{
			name:     "Node.js runtime",
			json:     `{"runtime":"nodejs20.x","memorySize":512}`,
			expected: "nodejs20.x",
		},
		{
			name:     "Java runtime",
			json:     `{"runtime":"java21","memorySize":1024}`,
			expected: "java21",
		},
		{
			name:     "Custom runtime provided.al2023",
			json:     `{"runtime":"provided.al2023","memorySize":128}`,
			expected: "provided.al2023",
		},
		{
			name:     "Custom runtime provided.al2",
			json:     `{"runtime":"provided.al2","memorySize":128}`,
			expected: "provided.al2",
		},
		{
			name:     "Dotnet runtime",
			json:     `{"runtime":"dotnet8","memorySize":512}`,
			expected: "dotnet8",
		},
		{
			name:     "Ruby runtime",
			json:     `{"runtime":"ruby3.3","memorySize":256}`,
			expected: "ruby3.3",
		},
		{
			name:     "No runtime field",
			json:     `{"memorySize":256}`,
			expected: "",
		},
		{
			name:     "Empty JSON",
			json:     "",
			expected: "",
		},
		{
			name:     "Invalid JSON",
			json:     "not json",
			expected: "",
		},
		{
			name:     "Runtime is not a string",
			json:     `{"runtime":123}`,
			expected: "",
		},
		{
			name:     "Runtime with whitespace",
			json:     `{"runtime":"  python3.12  "}`,
			expected: "python3.12",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractLambdaRuntime(tt.json)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseResourceRow_Lambda(t *testing.T) {
	cfg := config.ResourceConfig{
		ID:            "lambda",
		Type:          "lambda",
		CloudProvider: "aws",
		Inventory: config.InventoryConfig{
			FieldMappings: map[string]string{
				"region":      "region",
				"account_id":  "cloudAccount.externalId",
				"name":        "name",
				"external_id": "externalId",
			},
		},
	}

	source := NewGenericInventorySource(&Client{}, &cfg, nil, nil)

	cols := columnIndex{
		colHeaderExternalID:      0,
		colHeaderName:            1,
		colHeaderAccountID:       2,
		colHeaderRegion:          3,
		colHeaderTags:            4,
		colHeaderGraphProperties: 5,
	}

	tagsJSON := `[{"key":"app","value":"my-function"},{"key":"brand","value":"brand-a"}]`

	row := []string{
		"arn:aws:lambda:us-east-1:123456789012:function:my-func", // external_id
		"my-func",      // name
		"123456789012", // account_id
		"us-east-1",    // region
		tagsJSON,       // tags
		`{"runtime":"python3.12","memorySize":256}`, // graphEntity.properties
	}

	ctx := context.Background()
	resource, err := source.parseResourceRow(ctx, cols, row)

	require.NoError(t, err)
	assert.Equal(t, "arn:aws:lambda:us-east-1:123456789012:function:my-func", resource.ID)
	assert.Equal(t, "my-func", resource.Name)
	assert.Equal(t, types.ResourceType("lambda"), resource.Type)
	assert.Equal(t, types.CloudProviderAWS, resource.CloudProvider)
	assert.Equal(t, "123456789012", resource.CloudAccountID)
	assert.Equal(t, "us-east-1", resource.CloudRegion)
	assert.Equal(t, "python3.12", resource.CurrentVersion)
	assert.Equal(t, "aws-lambda", resource.Engine)
	assert.Equal(t, "my-function", resource.Service)
	assert.Equal(t, "brand-a", resource.Brand)
}

func TestParseResourceRow_LambdaNoRuntime(t *testing.T) {
	cfg := config.ResourceConfig{
		ID:            "lambda",
		Type:          "lambda",
		CloudProvider: "aws",
		Inventory: config.InventoryConfig{
			FieldMappings: map[string]string{
				"region":      "region",
				"account_id":  "cloudAccount.externalId",
				"name":        "name",
				"external_id": "externalId",
			},
		},
	}

	source := NewGenericInventorySource(&Client{}, &cfg, nil, nil)

	cols := columnIndex{
		colHeaderExternalID:      0,
		colHeaderName:            1,
		colHeaderAccountID:       2,
		colHeaderRegion:          3,
		colHeaderTags:            4,
		colHeaderGraphProperties: 5,
	}

	row := []string{
		"arn:aws:lambda:us-east-1:123456789012:function:no-runtime",
		"no-runtime",
		"123456789012",
		"us-east-1",
		"[]",
		`{"memorySize":256}`, // No runtime field
	}

	ctx := context.Background()
	resource, err := source.parseResourceRow(ctx, cols, row)

	// Container-image Lambdas (runtime=null) are skipped — AWS doesn't EOL them
	require.NoError(t, err)
	assert.Nil(t, resource)
}

func TestGetRequiredColumns_Lambda(t *testing.T) {
	cfg := config.ResourceConfig{
		Type: "lambda",
		Inventory: config.InventoryConfig{
			FieldMappings: map[string]string{
				"region":      "region",
				"account_id":  "cloudAccount.externalId",
				"name":        "name",
				"external_id": "externalId",
			},
		},
	}

	source := NewGenericInventorySource(&Client{}, &cfg, nil, nil)
	columns := source.getRequiredColumns()

	// Should include graphEntity.properties for Lambda
	assert.Contains(t, columns, colHeaderGraphProperties)
}

func TestListResources_LambdaFixture(t *testing.T) {
	// End-to-end test using the Lambda CSV fixture data.
	// Fixture nativeTypes are "lambda" (exact match), matching production Wiz data.
	mockWizClient := new(MockWizClient)
	mockWizClient.On("GetAccessToken", mock.Anything).Return("test-token", nil)
	mockWizClient.On("GetReport", mock.Anything, "test-token", "lambda-report-id").
		Return(WizAPIFixtures.LambdaReport, nil)
	mockWizClient.On("DownloadReport", mock.Anything, WizAPIFixtures.LambdaReport.DownloadURL).
		Return(NewMockReadCloser(WizAPIFixtures.LambdaCSVData), nil)

	wizClient := NewClient(mockWizClient, time.Hour)

	reportIDs := map[string]string{"lambda": "lambda-report-id"}
	reportIDsJSON, _ := json.Marshal(reportIDs)
	os.Setenv("WIZ_REPORT_IDS", string(reportIDsJSON))
	defer os.Unsetenv("WIZ_REPORT_IDS")

	cfg := config.ResourceConfig{
		ID:            "lambda",
		Type:          "lambda",
		CloudProvider: "aws",
		Inventory: config.InventoryConfig{
			NativeTypePattern: "lambda",
			FieldMappings: map[string]string{
				"region":      "region",
				"account_id":  "cloudAccount.externalId",
				"name":        "name",
				"external_id": "externalId",
			},
		},
	}

	source := NewGenericInventorySource(wizClient, &cfg, nil, nil)

	resources, err := source.ListResources(context.Background(), types.ResourceTypeLambda)
	require.NoError(t, err)

	// 4 of 5 fixture rows have a runtime; the no-runtime row is skipped
	// because container-image Lambdas are out of scope for EOL detection.
	require.Len(t, resources, 4)

	// Verify runtime extraction for each resource
	runtimeMap := make(map[string]string)
	for _, r := range resources {
		runtimeMap[r.Name] = r.CurrentVersion
	}

	assert.Equal(t, "python3.8", runtimeMap["legacy-python38"])
	assert.Equal(t, "nodejs20.x", runtimeMap["billing-node20"])
	assert.Equal(t, "java21", runtimeMap["payments-java21"])
	assert.Equal(t, "provided.al2023", runtimeMap["custom-runtime"])
	// no-runtime-props is excluded — container-image Lambda, no EOL to track
	assert.Empty(t, runtimeMap["no-runtime-props"])

	// All returned resources should have engine "aws-lambda"
	for _, r := range resources {
		assert.Equal(t, "aws-lambda", r.Engine, "resource %s should have engine aws-lambda", r.Name)
	}

	mockWizClient.AssertExpectations(t)
}

func TestParseResourceRow_WithContextTime(t *testing.T) {
	cfg := config.ResourceConfig{
		ID:            "test",
		Type:          "aurora",
		CloudProvider: "aws",
	}

	source := NewGenericInventorySource(&Client{}, &cfg, nil, nil)

	cols := columnIndex{
		colHeaderExternalID: 0,
		colHeaderName:       1,
		colHeaderAccountID:  2,
	}

	row := []string{
		"test-id",
		"test-name",
		"123456789012",
	}

	expectedTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	ctx := context.WithValue(context.Background(), discoveredAtKey, expectedTime)

	resource, err := source.parseResourceRow(ctx, cols, row)

	require.NoError(t, err)
	assert.Equal(t, expectedTime, resource.DiscoveredAt)
}
