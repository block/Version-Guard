package config

import "github.com/pkg/errors"

// TransformsConfig declares the per-resource version/engine
// transformations that used to live as hardcoded carve-outs in the
// Wiz parser (Lambda runtime extraction, OpenSearch version
// normalization, EKS engine default, Aurora substring-based engine
// normalization). Migrating them into YAML means a new resource type
// with the same shape as one of these can be added with zero Go
// changes.
//
// The DSL is intentionally NOT a general expression language. Each
// field has a small fixed set of named operations and at most one
// operation per field may be configured at a time — the loader
// rejects multiple sibling ops to keep YAML readable and prevent
// silently-overlapping behavior. New cases will land as new named
// ops here, not as runtime-evaluated expressions.
type TransformsConfig struct {
	// Version transforms how the inventory parser populates
	// Resource.CurrentVersion. Without this block the parser uses
	// the raw value of the column mapped to "version".
	Version *VersionTransform `yaml:"version,omitempty"`

	// Engine transforms how the inventory parser populates
	// Resource.Engine. Without this block the parser lowercases and
	// trims whatever was read from the "engine" column.
	Engine *EngineTransform `yaml:"engine,omitempty"`
}

// VersionTransform names exactly one operation that produces the
// final version string. Adding a third version-producing case in the
// future means adding a third field here, not generalizing the
// existing two.
//
//nolint:govet // field alignment sacrificed for readability
type VersionTransform struct {
	// StripPrefixes removes any of the listed leading substrings
	// (first match wins). Empty list disables the op. Used by
	// OpenSearch to drop "OpenSearch_"/"Elasticsearch_" prefixes
	// from the raw versionDetails.version column.
	StripPrefixes []string `yaml:"strip_prefixes,omitempty"`

	// ExtractJSONField parses a JSON column and reads a single
	// top-level field as the version. Used by Lambda to read the
	// "runtime" key out of graphEntity.properties.
	ExtractJSONField *ExtractJSONFieldOp `yaml:"extract_json_field,omitempty"`
}

// ExtractJSONFieldOp pulls a single string field out of a
// JSON-encoded column.
//
// SkipIfEmpty=true means a row whose extracted value is empty,
// missing, or non-string is dropped from the inventory result rather
// than passed downstream with an empty version. Lambda relies on
// this to silently exclude container-image functions (where
// runtime=null because AWS doesn't EOL container images).
type ExtractJSONFieldOp struct {
	FromColumn  string `yaml:"from_column"`
	Field       string `yaml:"field"`
	SkipIfEmpty bool   `yaml:"skip_if_empty,omitempty"`
}

// EngineTransform names exactly one engine-producing operation. The
// four ops cover the four shapes the existing carve-outs needed; new
// shapes land as new ops, not as expressions composed from these.
//
//nolint:govet // field alignment sacrificed for readability
type EngineTransform struct {
	// Constant overrides the engine to a fixed value regardless of
	// the column reading. Used by Lambda (always "aws-lambda").
	Constant string `yaml:"constant,omitempty"`

	// DefaultIfEmpty sets the engine to a constant when no engine
	// column is mapped (or the column is empty). Used by EKS
	// (always "eks") since EKS reports have no engine column.
	DefaultIfEmpty string `yaml:"default_if_empty,omitempty"`

	// SubstringLookup runs each rule in order against a lowercased
	// engine string; the first rule whose every Contains substring
	// is present wins. If no rule matches, the lowercased engine is
	// returned unchanged. Used by Aurora to map "AuroraMySQL" →
	// "aurora-mysql" etc.
	SubstringLookup []SubstringLookupRule `yaml:"substring_lookup,omitempty"`

	// FromVersionMajor derives the engine from the major version
	// (the segment before the first "."). Used by OpenSearch to
	// recognize legacy Elasticsearch installs (5/6/7) vs modern
	// OpenSearch (1/2/3+). Default is the fallback engine when no
	// major matches.
	FromVersionMajor *FromVersionMajorOp `yaml:"from_version_major,omitempty"`
}

// SubstringLookupRule says "if every entry in Contains appears in
// the input, return Result". Rules are evaluated in YAML order.
//
//nolint:govet // field alignment sacrificed for readability
type SubstringLookupRule struct {
	Contains []string `yaml:"contains"`
	Result   string   `yaml:"result"`
}

// FromVersionMajorOp maps a major version string to an engine name.
// Default is used when the major isn't in the map (or the version
// is empty).
type FromVersionMajorOp struct {
	Majors  map[string]string `yaml:"majors"`
	Default string            `yaml:"default"`
}

// validate enforces "at most one named op per field". This catches
// the common copy-paste mistake of leaving an old op in place when
// switching to a new one — without it the parser would silently
// pick a precedence and the YAML would be misleading to read.
func (t *TransformsConfig) validate() error {
	if t == nil {
		return nil
	}
	if err := t.Version.validate(); err != nil {
		return errors.Wrap(err, "transforms.version")
	}
	if err := t.Engine.validate(); err != nil {
		return errors.Wrap(err, "transforms.engine")
	}
	return nil
}

func (v *VersionTransform) validate() error {
	if v == nil {
		return nil
	}
	count := 0
	if len(v.StripPrefixes) > 0 {
		count++
	}
	if v.ExtractJSONField != nil {
		count++
	}
	if count > 1 {
		return errors.New("set at most one of strip_prefixes, extract_json_field")
	}
	if v.ExtractJSONField != nil {
		if v.ExtractJSONField.FromColumn == "" {
			return errors.New("extract_json_field.from_column is required")
		}
		if v.ExtractJSONField.Field == "" {
			return errors.New("extract_json_field.field is required")
		}
	}
	return nil
}

func (e *EngineTransform) validate() error {
	if e == nil {
		return nil
	}
	count := 0
	if e.Constant != "" {
		count++
	}
	if e.DefaultIfEmpty != "" {
		count++
	}
	if len(e.SubstringLookup) > 0 {
		count++
	}
	if e.FromVersionMajor != nil {
		count++
	}
	if count > 1 {
		return errors.New("set at most one of constant, default_if_empty, substring_lookup, from_version_major")
	}
	for i, rule := range e.SubstringLookup {
		if len(rule.Contains) == 0 {
			return errors.Errorf("substring_lookup[%d].contains must not be empty", i)
		}
		if rule.Result == "" {
			return errors.Errorf("substring_lookup[%d].result is required", i)
		}
	}
	if e.FromVersionMajor != nil {
		if len(e.FromVersionMajor.Majors) == 0 {
			return errors.New("from_version_major.majors must not be empty")
		}
		if e.FromVersionMajor.Default == "" {
			return errors.New("from_version_major.default is required")
		}
	}
	return nil
}
