package wiz

import (
	"encoding/json"
	"strings"

	"github.com/block/Version-Guard/pkg/config"
)

// applyVersionTransform produces the final version string for a row.
//
// rawVersion is the value already read from the column mapped to
// "version" — empty when the resource doesn't map a version column
// (Lambda's case, where the runtime is in graphEntity.properties
// instead).
//
// columnReader is supplied so the JSON-extraction path can pull a
// different column without the transforms layer needing to know
// about CSV column types.
//
// The returned skip flag means "drop this row entirely" — used by
// Lambda to silently exclude container-image functions whose runtime
// field is null/missing/empty (AWS doesn't EOL container-image
// Lambdas, so they belong out of scope rather than reported as
// findings with empty versions).
func applyVersionTransform(
	rawVersion string,
	t *config.VersionTransform,
	columnReader func(name string) string,
) (version string, skip bool) {
	if t == nil {
		return rawVersion, false
	}

	if t.ExtractJSONField != nil {
		extracted := extractJSONStringField(
			columnReader(t.ExtractJSONField.FromColumn),
			t.ExtractJSONField.Field,
		)
		if extracted == "" && t.ExtractJSONField.SkipIfEmpty {
			return "", true
		}
		return extracted, false
	}

	if len(t.StripPrefixes) > 0 {
		return stripFirstMatchingPrefix(rawVersion, t.StripPrefixes), false
	}

	return rawVersion, false
}

// applyEngineTransform produces the final engine string for a row.
// rawEngine is whatever was read from the column mapped to "engine"
// (or "" if no such column was mapped). version is the post-transform
// version from applyVersionTransform — needed by from_version_major,
// which derives the engine from the version's major component.
//
// Baseline behavior when no transform is set: lowercase + trim. This
// preserves the previous Go-side normalizeEngine baseline so resources
// that never had carve-outs (RDS, ElastiCache) keep producing the
// same lowercased engine strings without needing a no-op YAML block.
func applyEngineTransform(rawEngine, version string, t *config.EngineTransform) string {
	normalized := strings.ToLower(strings.TrimSpace(rawEngine))

	if t == nil {
		return normalized
	}

	if t.Constant != "" {
		return t.Constant
	}

	if t.DefaultIfEmpty != "" && normalized == "" {
		return t.DefaultIfEmpty
	}

	if t.FromVersionMajor != nil {
		return engineFromVersionMajor(version, t.FromVersionMajor)
	}

	if len(t.SubstringLookup) > 0 {
		for _, rule := range t.SubstringLookup {
			if substringRuleMatches(normalized, rule.Contains) {
				return rule.Result
			}
		}
	}

	return normalized
}

// extractJSONStringField parses raw as JSON and returns the named
// top-level field's string value. Returns "" when raw is empty or
// unparseable, when the field is missing, or when its value isn't a
// string. The space-trim mirrors AWS's tendency to round-trip values
// with whitespace in tooling output.
func extractJSONStringField(raw, field string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var props map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &props); err != nil {
		return ""
	}
	val, ok := props[field].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(val)
}

// stripFirstMatchingPrefix returns s with the first matching prefix
// from prefixes removed. The prefixes we currently use are mutually
// exclusive (OpenSearch_/Elasticsearch_) but iterating "first match
// wins" is the principle of least surprise either way.
func stripFirstMatchingPrefix(s string, prefixes []string) string {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return strings.TrimPrefix(s, p)
		}
	}
	return s
}

// substringRuleMatches reports whether every entry in contains
// appears in input. Empty contains is treated as no-match to avoid
// silently matching every row when YAML is misconfigured (the loader
// also rejects this case at startup).
func substringRuleMatches(input string, contains []string) bool {
	if len(contains) == 0 {
		return false
	}
	for _, sub := range contains {
		if !strings.Contains(input, sub) {
			return false
		}
	}
	return true
}

// engineFromVersionMajor reads the segment before the first '.' from
// version and looks it up in op.Majors. Empty version or a major
// that's not in the map falls back to op.Default.
func engineFromVersionMajor(version string, op *config.FromVersionMajorOp) string {
	if version == "" {
		return op.Default
	}
	major := strings.SplitN(version, ".", 2)[0]
	if engine, ok := op.Majors[major]; ok {
		return engine
	}
	return op.Default
}
