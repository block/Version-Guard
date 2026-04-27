package config

import (
	"os"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

// LoadResourcesConfig loads and parses the resources configuration file
func LoadResourcesConfig(path string) (*ResourcesConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read config file: %s", path)
	}

	var config ResourcesConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, errors.Wrap(err, "failed to parse YAML config")
	}

	// Validate config
	if err := validateConfig(&config); err != nil {
		return nil, errors.Wrap(err, "invalid configuration")
	}

	return &config, nil
}

// validateConfig validates the resources configuration
func validateConfig(config *ResourcesConfig) error {
	if config.Version == "" {
		return errors.New("version is required")
	}

	for i := range config.Resources {
		resource := &config.Resources[i]
		if resource.ID == "" {
			return errors.Errorf("resource[%d]: id is required", i)
		}
		if resource.Type == "" {
			return errors.Errorf("resource[%d]: type is required", i)
		}
		if resource.CloudProvider == "" {
			return errors.Errorf("resource[%d]: cloud_provider is required", i)
		}
		if resource.Inventory.Source == "" {
			return errors.Errorf("resource[%d]: inventory.source is required", i)
		}
		if resource.EOL.Provider == "" {
			return errors.Errorf("resource[%d]: eol.provider is required", i)
		}
		if resource.EOL.Product == "" {
			return errors.Errorf("resource[%d]: eol.product is required", i)
		}
		if err := validateFieldMappings(resource); err != nil {
			return errors.Wrapf(err, "resource[%d] %q", i, resource.ID)
		}
	}

	return nil
}

// validateFieldMappings ensures that resource_id, version, and engine
// are present in the resource's field_mappings.
//
// Some resource types are exempt because they don't expose a single CSV
// column for one or more of those fields:
//   - lambda:     version (runtime) is extracted from graphEntity.properties
//     JSON, and engine is always "aws-lambda".
//   - eks:        engine is hard-coded to "eks" since EKS reports don't
//     include an engine column.
//   - opensearch: engine is derived from the version (legacy
//     Elasticsearch versions vs OpenSearch).
func validateFieldMappings(resource *ResourceConfig) error {
	required := []string{"resource_id"}
	switch resource.Type {
	case "lambda":
		// version & engine derived from graphEntity.properties JSON
	case "eks", "opensearch":
		// engine is implicit
		required = append(required, "version")
	default:
		required = append(required, "version", "engine")
	}

	for _, key := range required {
		if v := resource.Inventory.FieldMappings[key]; v == "" {
			return errors.Errorf("inventory.field_mappings.%s is required", key)
		}
	}

	return nil
}
