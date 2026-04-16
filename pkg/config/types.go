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

// InventoryConfig defines inventory source configuration
type InventoryConfig struct {
	Source             string                 `yaml:"source"`
	ReportIDEnv        string                 `yaml:"report_id_env"`
	NativeTypePattern  string                 `yaml:"native_type_pattern"`
	FieldMappings      map[string]string      `yaml:"field_mappings"`
}

// EOLConfig defines EOL provider configuration
type EOLConfig struct {
	Provider string `yaml:"provider"`
	Product  string `yaml:"product"`
	Schema   string `yaml:"schema"`
}
