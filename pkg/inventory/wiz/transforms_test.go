package wiz

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/block/Version-Guard/pkg/config"
)

// TestApplyVersionTransform exercises the YAML-declared version
// reshaping previously hardcoded as carve-outs in parseResourceRow:
//
//   - extract_json_field: Lambda runtime extraction from
//     graphEntity.properties JSON, with skip_if_empty for
//     container-image Lambdas (out of scope, not findings).
//   - strip_prefixes:    OpenSearch prefix stripping
//     ("OpenSearch_2.13" → "2.13").
//
// The "no transform" case is included to guarantee resources without a
// transforms block (RDS/ElastiCache/Aurora) still see the raw column
// value flow through unchanged.
func TestApplyVersionTransform(t *testing.T) {
	tests := []struct {
		name        string
		raw         string
		transform   *config.VersionTransform
		columns     map[string]string
		wantVersion string
		wantSkip    bool
	}{
		{
			name:        "nil transform passes raw through",
			raw:         "8.0.34",
			transform:   nil,
			wantVersion: "8.0.34",
		},
		{
			name: "strip_prefixes removes OpenSearch_",
			raw:  "OpenSearch_2.13",
			transform: &config.VersionTransform{
				StripPrefixes: []string{"OpenSearch_", "Elasticsearch_"},
			},
			wantVersion: "2.13",
		},
		{
			name: "strip_prefixes removes Elasticsearch_",
			raw:  "Elasticsearch_7.10",
			transform: &config.VersionTransform{
				StripPrefixes: []string{"OpenSearch_", "Elasticsearch_"},
			},
			wantVersion: "7.10",
		},
		{
			name: "strip_prefixes leaves untouched if no match",
			raw:  "1.2.3",
			transform: &config.VersionTransform{
				StripPrefixes: []string{"OpenSearch_"},
			},
			wantVersion: "1.2.3",
		},
		{
			name: "extract_json_field reads runtime from JSON column",
			transform: &config.VersionTransform{
				ExtractJSONField: &config.ExtractJSONFieldOp{
					FromColumn:  "graphEntity.properties",
					Field:       "runtime",
					SkipIfEmpty: true,
				},
			},
			columns:     map[string]string{"graphEntity.properties": `{"runtime":"python3.12","memorySize":256}`},
			wantVersion: "python3.12",
		},
		{
			name: "extract_json_field with skip_if_empty drops row when JSON has no field",
			transform: &config.VersionTransform{
				ExtractJSONField: &config.ExtractJSONFieldOp{
					FromColumn:  "graphEntity.properties",
					Field:       "runtime",
					SkipIfEmpty: true,
				},
			},
			columns:  map[string]string{"graphEntity.properties": `{"memorySize":256}`},
			wantSkip: true,
		},
		{
			name: "extract_json_field with skip_if_empty drops row when column is empty",
			transform: &config.VersionTransform{
				ExtractJSONField: &config.ExtractJSONFieldOp{
					FromColumn:  "graphEntity.properties",
					Field:       "runtime",
					SkipIfEmpty: true,
				},
			},
			columns:  map[string]string{"graphEntity.properties": ""},
			wantSkip: true,
		},
		{
			name: "extract_json_field without skip_if_empty returns empty version, no skip",
			transform: &config.VersionTransform{
				ExtractJSONField: &config.ExtractJSONFieldOp{
					FromColumn:  "graphEntity.properties",
					Field:       "runtime",
					SkipIfEmpty: false,
				},
			},
			columns:     map[string]string{"graphEntity.properties": "{}"},
			wantVersion: "",
			wantSkip:    false,
		},
		{
			name: "extract_json_field handles invalid JSON gracefully",
			transform: &config.VersionTransform{
				ExtractJSONField: &config.ExtractJSONFieldOp{
					FromColumn:  "graphEntity.properties",
					Field:       "runtime",
					SkipIfEmpty: true,
				},
			},
			columns:  map[string]string{"graphEntity.properties": "not json"},
			wantSkip: true,
		},
		{
			name: "extract_json_field rejects non-string field values",
			transform: &config.VersionTransform{
				ExtractJSONField: &config.ExtractJSONFieldOp{
					FromColumn:  "graphEntity.properties",
					Field:       "runtime",
					SkipIfEmpty: true,
				},
			},
			columns:  map[string]string{"graphEntity.properties": `{"runtime":123}`},
			wantSkip: true,
		},
		{
			name: "extract_json_field trims whitespace in extracted value",
			transform: &config.VersionTransform{
				ExtractJSONField: &config.ExtractJSONFieldOp{
					FromColumn: "graphEntity.properties",
					Field:      "runtime",
				},
			},
			columns:     map[string]string{"graphEntity.properties": `{"runtime":"  python3.12  "}`},
			wantVersion: "python3.12",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version, skip := applyVersionTransform(
				tt.raw,
				tt.transform,
				func(name string) string { return tt.columns[name] },
			)
			assert.Equal(t, tt.wantVersion, version)
			assert.Equal(t, tt.wantSkip, skip)
		})
	}
}

// TestApplyEngineTransform exercises every engine-shape op:
//
//   - constant:           Lambda always sets engine to "aws-lambda".
//   - default_if_empty:   EKS reports have no engine column; default
//     to "eks".
//   - substring_lookup:   Aurora maps "AuroraMySQL"/"AuroraPostgreSQL"
//     to canonical engine names. Lowercase+trim
//     is applied before matching, so YAML rules
//     can stay all-lowercase.
//   - from_version_major: OpenSearch picks "elasticsearch" for ES
//     5/6/7 (legacy) vs "opensearch" for 1/2/3+.
//
// The "no transform" baseline (lowercase+trim) is exercised too to
// pin behavior for resources without quirks.
func TestApplyEngineTransform(t *testing.T) {
	auroraRules := []config.SubstringLookupRule{
		{Contains: []string{"aurora", "mysql"}, Result: "aurora-mysql"},
		{Contains: []string{"aurora", "postgres"}, Result: "aurora-postgresql"},
	}
	openSearchOp := &config.FromVersionMajorOp{
		Majors:  map[string]string{"5": "elasticsearch", "6": "elasticsearch", "7": "elasticsearch"},
		Default: "opensearch",
	}

	tests := []struct {
		name      string
		raw       string
		version   string
		transform *config.EngineTransform
		want      string
	}{
		{
			name:      "nil transform lowercases and trims (RDS/ElastiCache baseline)",
			raw:       "  Redis ",
			transform: nil,
			want:      "redis",
		},
		{
			name:      "constant always wins (Lambda)",
			raw:       "ignored-engine-column-value",
			transform: &config.EngineTransform{Constant: "aws-lambda"},
			want:      "aws-lambda",
		},
		{
			name:      "default_if_empty fires when engine is empty (EKS, no column)",
			raw:       "",
			transform: &config.EngineTransform{DefaultIfEmpty: "eks"},
			want:      "eks",
		},
		{
			name:      "default_if_empty does NOT override a present value",
			raw:       "Redis",
			transform: &config.EngineTransform{DefaultIfEmpty: "eks"},
			want:      "redis",
		},
		{
			name:      "substring_lookup maps Aurora MySQL",
			raw:       "AuroraMySQL",
			transform: &config.EngineTransform{SubstringLookup: auroraRules},
			want:      "aurora-mysql",
		},
		{
			name:      "substring_lookup maps Aurora PostgreSQL",
			raw:       "AuroraPostgreSQL",
			transform: &config.EngineTransform{SubstringLookup: auroraRules},
			want:      "aurora-postgresql",
		},
		{
			name:      "substring_lookup falls through to lowercased input on miss",
			raw:       "Memcached",
			transform: &config.EngineTransform{SubstringLookup: auroraRules},
			want:      "memcached",
		},
		{
			name:      "from_version_major picks elasticsearch for legacy 7.x",
			version:   "7.10",
			transform: &config.EngineTransform{FromVersionMajor: openSearchOp},
			want:      "elasticsearch",
		},
		{
			name:      "from_version_major picks elasticsearch for 5.x",
			version:   "5.6",
			transform: &config.EngineTransform{FromVersionMajor: openSearchOp},
			want:      "elasticsearch",
		},
		{
			name:      "from_version_major picks opensearch for modern 2.x",
			version:   "2.13",
			transform: &config.EngineTransform{FromVersionMajor: openSearchOp},
			want:      "opensearch",
		},
		{
			name:      "from_version_major falls back to default on empty version",
			version:   "",
			transform: &config.EngineTransform{FromVersionMajor: openSearchOp},
			want:      "opensearch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyEngineTransform(tt.raw, tt.version, tt.transform)
			assert.Equal(t, tt.want, got)
		})
	}
}
