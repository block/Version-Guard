package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadResourcesConfig_Success(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "resources.yaml")

	configContent := `version: v1
resources:
  - id: aurora-postgresql
    type: aurora
    cloud_provider: aws
    inventory:
      source: wiz
      native_type_pattern: "rds/AmazonAuroraPostgreSQL/cluster"
      required_mappings:
        resource_id: "externalId"
        version: "versionDetails.version"
        engine: "typeFields.kind"
      field_mappings:
        name: "name"
        account_id: "cloudAccount.externalId"
        region: "region"
    eol:
      provider: endoflife-date
      product: amazon-aurora-postgresql
      schema: standard
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	// Load the config
	cfg, err := LoadResourcesConfig(configFile)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify config structure
	assert.Equal(t, "v1", cfg.Version)
	assert.Len(t, cfg.Resources, 1)

	// Verify resource details
	res := cfg.Resources[0]
	assert.Equal(t, "aurora-postgresql", res.ID)
	assert.Equal(t, "aurora", res.Type)
	assert.Equal(t, "aws", res.CloudProvider)

	// Verify inventory config
	assert.Equal(t, "wiz", res.Inventory.Source)
	assert.Equal(t, "rds/AmazonAuroraPostgreSQL/cluster", res.Inventory.NativeTypePattern)
	assert.Len(t, res.Inventory.RequiredMappings, 3)
	assert.Equal(t, "externalId", res.Inventory.RequiredMappings["resource_id"])
	assert.Equal(t, "typeFields.kind", res.Inventory.RequiredMappings["engine"])
	assert.Equal(t, "versionDetails.version", res.Inventory.RequiredMappings["version"])
	assert.Len(t, res.Inventory.FieldMappings, 3)
	assert.Equal(t, "name", res.Inventory.FieldMappings["name"])

	// Verify EOL config
	assert.Equal(t, "endoflife-date", res.EOL.Provider)
	assert.Equal(t, "amazon-aurora-postgresql", res.EOL.Product)
	assert.Equal(t, "standard", res.EOL.Schema)
}

func TestLoadResourcesConfig_MultipleResources(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "resources.yaml")

	configContent := `version: v1
resources:
  - id: aurora-postgresql
    type: aurora
    cloud_provider: aws
    inventory:
      source: wiz
      native_type_pattern: "rds/AmazonAuroraPostgreSQL/cluster"
      required_mappings:
        resource_id: "externalId"
        version: "versionDetails.version"
        engine: "typeFields.kind"
    eol:
      provider: endoflife-date
      product: amazon-aurora-postgresql
      schema: standard
  - id: eks
    type: eks
    cloud_provider: aws
    inventory:
      source: wiz
      native_type_pattern: "eks/Cluster"
      required_mappings:
        resource_id: "providerUniqueId"
        version: "versionDetails.version"
    eol:
      provider: endoflife-date
      product: amazon-eks
      schema: eks_adapter
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg, err := LoadResourcesConfig(configFile)
	require.NoError(t, err)
	assert.Len(t, cfg.Resources, 2)

	// Verify both resources
	assert.Equal(t, "aurora-postgresql", cfg.Resources[0].ID)
	assert.Equal(t, "eks", cfg.Resources[1].ID)
	assert.Equal(t, "standard", cfg.Resources[0].EOL.Schema)
	assert.Equal(t, "eks_adapter", cfg.Resources[1].EOL.Schema)
}

func TestLoadResourcesConfig_FileNotFound(t *testing.T) {
	cfg, err := LoadResourcesConfig("/nonexistent/file.yaml")
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "failed to read config file")
}

func TestLoadResourcesConfig_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "invalid.yaml")

	invalidContent := `version: v1
resources:
  - id: test
    invalid yaml here [[[
`

	err := os.WriteFile(configFile, []byte(invalidContent), 0644)
	require.NoError(t, err)

	cfg, err := LoadResourcesConfig(configFile)
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "failed to parse YAML config")
}

func TestValidateConfig_MissingVersion(t *testing.T) {
	cfg := &ResourcesConfig{
		Version: "",
		Resources: []ResourceConfig{
			{
				ID:            "test",
				Type:          "aurora",
				CloudProvider: "aws",
				Inventory: InventoryConfig{
					Source: "wiz",
				},
				EOL: EOLConfig{
					Provider: "endoflife-date",
					Product:  "test",
				},
			},
		},
	}

	err := validateConfig(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "version is required")
}

func TestValidateConfig_MissingResourceID(t *testing.T) {
	cfg := &ResourcesConfig{
		Version: "v1",
		Resources: []ResourceConfig{
			{
				ID:            "", // Missing
				Type:          "aurora",
				CloudProvider: "aws",
				Inventory: InventoryConfig{
					Source: "wiz",
				},
				EOL: EOLConfig{
					Provider: "endoflife-date",
					Product:  "test",
				},
			},
		},
	}

	err := validateConfig(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "id is required")
}

func TestValidateConfig_MissingType(t *testing.T) {
	cfg := &ResourcesConfig{
		Version: "v1",
		Resources: []ResourceConfig{
			{
				ID:            "test",
				Type:          "", // Missing
				CloudProvider: "aws",
				Inventory: InventoryConfig{
					Source: "wiz",
				},
				EOL: EOLConfig{
					Provider: "endoflife-date",
					Product:  "test",
				},
			},
		},
	}

	err := validateConfig(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "type is required")
}

func TestValidateConfig_MissingCloudProvider(t *testing.T) {
	cfg := &ResourcesConfig{
		Version: "v1",
		Resources: []ResourceConfig{
			{
				ID:            "test",
				Type:          "aurora",
				CloudProvider: "", // Missing
				Inventory: InventoryConfig{
					Source: "wiz",
				},
				EOL: EOLConfig{
					Provider: "endoflife-date",
					Product:  "test",
				},
			},
		},
	}

	err := validateConfig(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cloud_provider is required")
}

func TestValidateConfig_MissingInventorySource(t *testing.T) {
	cfg := &ResourcesConfig{
		Version: "v1",
		Resources: []ResourceConfig{
			{
				ID:            "test",
				Type:          "aurora",
				CloudProvider: "aws",
				Inventory: InventoryConfig{
					Source: "", // Missing
				},
				EOL: EOLConfig{
					Provider: "endoflife-date",
					Product:  "test",
				},
			},
		},
	}

	err := validateConfig(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "inventory.source is required")
}

func TestValidateConfig_MissingEOLProvider(t *testing.T) {
	cfg := &ResourcesConfig{
		Version: "v1",
		Resources: []ResourceConfig{
			{
				ID:            "test",
				Type:          "aurora",
				CloudProvider: "aws",
				Inventory: InventoryConfig{
					Source: "wiz",
				},
				EOL: EOLConfig{
					Provider: "", // Missing
					Product:  "test",
				},
			},
		},
	}

	err := validateConfig(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "eol.provider is required")
}

// TestValidateConfig_Mappings exercises the required_mappings /
// field_mappings split: each resource self-declares what's required,
// every value in required_mappings must be non-empty, and a key may
// appear in at most one of the two maps.
func TestValidateConfig_Mappings(t *testing.T) {
	tests := []struct {
		name       string
		required   map[string]string
		field      map[string]string
		wantErrSub string
	}{
		{
			name:       "missing resource_id fails",
			required:   map[string]string{"version": "v", "engine": "e"},
			wantErrSub: "required_mappings.resource_id is required",
		},
		{
			name:       "empty resource_id fails",
			required:   map[string]string{"resource_id": "", "version": "v", "engine": "e"},
			wantErrSub: "required_mappings.resource_id is required",
		},
		{
			name:       "empty value in required_mappings fails",
			required:   map[string]string{"resource_id": "id", "version": "v", "engine": ""},
			wantErrSub: "required_mappings.engine must not be empty",
		},
		{
			name:       "duplicate key in both maps fails",
			required:   map[string]string{"resource_id": "id", "version": "v"},
			field:      map[string]string{"version": "v2"},
			wantErrSub: `mapping "version" appears in both`,
		},
		{
			name: "minimal valid config (Lambda-style: resource_id only)",
			required: map[string]string{
				"resource_id": "externalId",
			},
		},
		{
			name: "EKS-style: resource_id + version only (engine implicit)",
			required: map[string]string{
				"resource_id": "providerUniqueId",
				"version":     "versionDetails.version",
			},
		},
		{
			name: "Aurora-style: resource_id + version + engine",
			required: map[string]string{
				"resource_id": "externalId",
				"version":     "versionDetails.version",
				"engine":      "typeFields.kind",
			},
			field: map[string]string{
				"name":       "name",
				"account_id": "cloudAccount.externalId",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &ResourcesConfig{
				Version: "v1",
				Resources: []ResourceConfig{
					{
						ID:            "test",
						Type:          "aurora",
						CloudProvider: "aws",
						Inventory: InventoryConfig{
							Source:           "wiz",
							RequiredMappings: tt.required,
							FieldMappings:    tt.field,
						},
						EOL: EOLConfig{
							Provider: "endoflife-date",
							Product:  "test",
						},
					},
				},
			}

			err := validateConfig(cfg)
			if tt.wantErrSub == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrSub)
			}
		})
	}
}

func TestValidateConfig_MissingEOLProduct(t *testing.T) {
	cfg := &ResourcesConfig{
		Version: "v1",
		Resources: []ResourceConfig{
			{
				ID:            "test",
				Type:          "aurora",
				CloudProvider: "aws",
				Inventory: InventoryConfig{
					Source: "wiz",
				},
				EOL: EOLConfig{
					Provider: "endoflife-date",
					Product:  "", // Missing
				},
			},
		},
	}

	err := validateConfig(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "eol.product is required")
}
