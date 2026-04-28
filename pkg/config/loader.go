package config

import (
	"os"
	"sort"
	"strings"

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
		if err := validateMappings(resource); err != nil {
			return errors.Wrapf(err, "resource[%d] %q", i, resource.ID)
		}
	}

	return nil
}

// validateMappings enforces three rules on a resource's
// required_mappings / field_mappings split:
//
//  1. resource_id MUST appear in required_mappings — the system
//     can't function without a primary key for each row.
//  2. Every value in required_mappings must be non-empty. Listing a
//     key in required_mappings is the YAML way of asserting it is
//     mandatory; an empty string means the user forgot to fill it in.
//  3. A key may appear in required_mappings OR field_mappings but
//     never both. Duplicates indicate a copy-paste mistake and would
//     hide intent from the next reader.
//
// What's "required" is per-resource and self-declared in YAML. We
// no longer carry a Go-side switch on resource.Type because the
// previous carve-outs (lambda derives version/engine implicitly from
// graphEntity.properties; eks defaults engine to "eks"; opensearch
// derives engine from the version) are simply expressed as different
// required_mappings sets in the config file.
func validateMappings(resource *ResourceConfig) error {
	inv := &resource.Inventory

	id, ok := inv.RequiredMappings["resource_id"]
	if !ok || id == "" {
		return errors.New("inventory.required_mappings.resource_id is required")
	}

	for key, val := range inv.RequiredMappings {
		if val == "" {
			return errors.Errorf("inventory.required_mappings.%s must not be empty", key)
		}
	}

	for key := range inv.RequiredMappings {
		if _, dup := inv.FieldMappings[key]; dup {
			return errors.Errorf(
				"mapping %q appears in both inventory.required_mappings and inventory.field_mappings; pick one",
				key,
			)
		}
	}

	return nil
}

// FilterResources returns a copy of cfg with Resources narrowed to the
// IDs listed in ids. ids is whitespace-trimmed and deduplicated; an
// empty slice (after trimming) is treated as "no filter" and the
// original config is returned unchanged.
//
// Any ID that doesn't match a resource in cfg is reported as an error,
// listing every unknown ID at once so users can fix typos in a single
// pass instead of one per run.
func FilterResources(cfg *ResourcesConfig, ids []string) (*ResourcesConfig, error) {
	wanted := make(map[string]struct{}, len(ids))
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		wanted[id] = struct{}{}
	}
	if len(wanted) == 0 {
		return cfg, nil
	}

	known := make(map[string]struct{}, len(cfg.Resources))
	filtered := make([]ResourceConfig, 0, len(wanted))
	for i := range cfg.Resources {
		r := cfg.Resources[i]
		known[r.ID] = struct{}{}
		if _, ok := wanted[r.ID]; ok {
			filtered = append(filtered, r)
		}
	}

	var unknown []string
	for id := range wanted {
		if _, ok := known[id]; !ok {
			unknown = append(unknown, id)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		availableList := make([]string, 0, len(known))
		for id := range known {
			availableList = append(availableList, id)
		}
		sort.Strings(availableList)
		return nil, errors.Errorf(
			"unknown resource id(s): %s (known: %s)",
			strings.Join(unknown, ", "),
			strings.Join(availableList, ", "),
		)
	}

	out := *cfg
	out.Resources = filtered
	return &out, nil
}
