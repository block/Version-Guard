package config

// ResourcesConfig represents the root configuration structure
type ResourcesConfig struct {
	Version   string           `yaml:"version"`
	Resources []ResourceConfig `yaml:"resources"`
}

// ResourceConfig defines configuration for a single resource type
type ResourceConfig struct {
	ID            string          `yaml:"id"`
	Type          string          `yaml:"type"`
	CloudProvider string          `yaml:"cloud_provider"`
	Inventory     InventoryConfig `yaml:"inventory"`
	EOL           EOLConfig       `yaml:"eol"`
}

// InventoryConfig defines inventory source configuration.
//
// The mappings split into two sections to make the schema obvious:
//   - RequiredMappings (yaml: "required_mappings") contains the typed,
//     mandatory CSV columns that the parser writes into typed Resource
//     fields: resource_id, version, engine.
//   - FieldMappings (yaml: "field_mappings") contains every other CSV
//     column; values land in Resource.Fields keyed by the YAML logical
//     name. Users add a new field by adding a new key here — no code
//     changes required.
type InventoryConfig struct {
	RequiredMappings  map[string]string `yaml:"required_mappings"`
	FieldMappings     map[string]string `yaml:"field_mappings"`
	Source            string            `yaml:"source"`
	NativeTypePattern string            `yaml:"native_type_pattern"`
}

// EOLConfig defines EOL provider configuration
type EOLConfig struct {
	Provider string `yaml:"provider"`
	Product  string `yaml:"product"`
	Schema   string `yaml:"schema"`
}
