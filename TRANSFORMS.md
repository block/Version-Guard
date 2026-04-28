# Transforms — declarative reshaping of inventory data

Version Guard parses inventory data (Wiz CSV reports today, anything later) into a typed `Resource` with a small typed surface: identity, version, engine, service, tags, plus a free-form `Extra` map for everything the YAML wants to pass through. **Transforms** are the YAML-driven step that takes the raw values pulled from inventory columns and reshapes them into the `version` and `engine` fields the EOL pipeline understands.

This document explains:

1. [What problem transforms solve](#what-problem-transforms-solve)
2. [When to use a transform](#when-to-use-a-transform)
3. [When NOT to use a transform](#when-not-to-use-a-transform)
4. [The DSL at a glance](#the-dsl-at-a-glance)
5. [Operation reference](#operation-reference)
6. [Composition rules](#composition-rules)
7. [Validation guarantees](#validation-guarantees)
8. [Worked examples — the canonical resources](#worked-examples--the-canonical-resources)
9. [Anti-patterns](#anti-patterns)
10. [Adding a new operation](#adding-a-new-operation)

---

## What problem transforms solve

Inventory data is messy. Wiz reports a Lambda's runtime as a JSON-encoded blob in `graphEntity.properties`, not a column. OpenSearch versions arrive as `OpenSearch_2.13` or `Elasticsearch_7.10` because the engine and version are concatenated. EKS reports have no engine column at all. Aurora's engine column says `AuroraPostgreSQL`, but endoflife.date keys cycles by `aurora-postgresql`.

Before transforms, each of these quirks lived as a per-resource-type `if` branch in Go:

```go
if s.config.Type == "lambda" { extract from JSON, force engine = "aws-lambda" ... }
if s.config.Type == "eks"    { default engine to "eks" ... }
if s.config.Type == "opensearch" { strip prefix, derive engine from major ... }
// ...etc
```

That meant a new resource that needed any of these reshapes required a Go change, a build, a release. Transforms move every one of those reshapes into YAML so onboarding a new resource that fits any existing shape is **YAML-only**.

> The DSL is intentionally **not** a general expression language. Each field has a small fixed set of named operations. Adding a new shape means adding a new named op to `pkg/config/transforms.go` and a corresponding applier in `pkg/inventory/wiz/transforms.go` — never composing expressions at runtime.

---

## When to use a transform

Use a transform when:

- The inventory column for `version` or `engine` doesn't directly produce the value endoflife.date expects.
- The inventory data carries the version/engine inside a JSON blob rather than a flat column.
- The engine needs to be derived from the version (legacy Elasticsearch vs OpenSearch).
- The engine is implicit and should default to a constant (EKS clusters have no engine column).
- The engine is the same constant for every row of a resource type (Lambda → `aws-lambda`).

Use the smallest, most specific operation that captures the intent. If a built-in named op exists for the shape you have, use it. If not, **add a new named op** rather than reaching for a more general operation.

---

## When NOT to use a transform

Don't use a transform when:

- **The raw column value is already correct.** ElastiCache reports `Redis`/`Valkey`/`Memcached` — the no-transform baseline (lowercase + trim) produces `redis`/`valkey`/`memcached`, which is what endoflife.date wants. Adding a transform would be noise.
- **The reshape is one-off, never reused, and unlikely to recur.** Transforms are a vocabulary; every new operation should be plausibly useful for >1 resource (current or future). One-off code can live as a Go change.
- **The reshape depends on values outside the row** (e.g., joining two reports, looking up an account name). Transforms operate on a single inventory row. Cross-row logic belongs in the inventory source, not the DSL.
- **The validation needed is more than presence + structure.** The DSL validator only checks shape (required keys, mutually exclusive ops). It does not type-check version strings or pattern-match engine names. If you need richer validation, do it in Go and surface a `Resource` field.
- **You're tempted to compose two ops because none of them quite fits.** Resist. Add a new named op instead — see [Adding a new operation](#adding-a-new-operation).

---

## The DSL at a glance

```yaml
transforms:
  version:
    # Pick AT MOST ONE:
    strip_prefixes: [str, ...]
    extract_json_field:
      from_column: "<csv-column-name>"
      field: "<top-level-json-key>"
      skip_if_empty: true|false
  engine:
    # Pick AT MOST ONE:
    constant: "<value>"
    default_if_empty: "<value>"
    substring_lookup:
      - contains: [str, ...]
        result: "<value>"
      # ...more rules in priority order
    from_version_major:
      majors:
        "<major>": "<engine>"
      default: "<fallback-engine>"
```

| Field | Op | Args | Example use |
|---|---|---|---|
| `transforms.version` | `strip_prefixes` | `[str]` | OpenSearch (`OpenSearch_2.11` → `2.11`) |
| `transforms.version` | `extract_json_field` | `{from_column, field, skip_if_empty}` | Lambda (runtime out of `graphEntity.properties`) |
| `transforms.engine` | `constant` | `str` | Lambda (always `aws-lambda`) |
| `transforms.engine` | `default_if_empty` | `str` | EKS (no engine column → `eks`) |
| `transforms.engine` | `substring_lookup` | `[{contains, result}]` | Aurora (`AuroraPostgreSQL` → `aurora-postgresql`) |
| `transforms.engine` | `from_version_major` | `{majors, default}` | OpenSearch (5/6/7 → `elasticsearch`, else → `opensearch`) |

If a resource has no `transforms` block, the parser:

- Reads `version` verbatim from the column mapped to `"version"`.
- Reads `engine` from the column mapped to `"engine"`, then **lowercases and trims** it.

That baseline alone covers ElastiCache, RDS-MySQL, RDS-PostgreSQL.

---

## Operation reference

### Version operations

#### `strip_prefixes: [str, ...]`

Remove the first matching prefix from the raw version string. If no prefix matches, the value passes through unchanged. Useful when the inventory bundles the engine name into the version string.

```yaml
transforms:
  version:
    strip_prefixes: ["OpenSearch_", "Elasticsearch_"]
```

| Input | Output |
|---|---|
| `OpenSearch_2.13` | `2.13` |
| `Elasticsearch_7.10` | `7.10` |
| `1.2.3` | `1.2.3` (no prefix matched) |

**When to use:** the inventory pre-pends a fixed string set to the version.
**When not to use:** if the prefix isn't a fixed string (e.g. requires regex). Add a new op instead.

#### `extract_json_field`

Parse a single column as JSON and read a single top-level field. Used when the inventory packs structured data into one cell instead of giving you separate columns.

```yaml
transforms:
  version:
    extract_json_field:
      from_column: "graphEntity.properties"
      field: "runtime"
      skip_if_empty: true
```

- `from_column` — the inventory column to parse. The column is automatically added to the required-columns list, so the Wiz header validator catches typos at parse start (no need to also list it in `field_mappings`).
- `field` — the top-level JSON key to read. Only string values are accepted; non-string returns empty.
- `skip_if_empty` — if `true`, rows whose extracted value is empty/missing/null/non-string are **dropped from the inventory result entirely**. Used by Lambda to silently exclude container-image functions (where `runtime=null` because AWS doesn't EOL container images). If `false`, the row is kept with an empty `version`.

| `from_column` value | `field` | Output | Skip? |
|---|---|---|---|
| `{"runtime":"python3.12"}` | `runtime` | `python3.12` | no |
| `{"runtime":"  java21  "}` | `runtime` | `java21` (trimmed) | no |
| `{"memorySize":256}` | `runtime` | `""` | yes (if `skip_if_empty`) |
| `{"runtime":123}` | `runtime` | `""` | yes (if `skip_if_empty`) |
| `not json` | `runtime` | `""` | yes (if `skip_if_empty`) |

**When to use:** the inventory hides the value inside a JSON blob.
**When not to use:** if you need to read multiple JSON fields, or compute one from another. Today this op reads a single top-level string field; if you need more, add a new op.

### Engine operations

The engine column is **always** lowercased + trimmed before any operation runs. Substring rules can therefore be written in lowercase and avoid casing concerns. The no-transform default is exactly this lowercase+trim — substring/from-version-major/default-if-empty all preserve it.

#### `constant: "<value>"`

Force the engine to a fixed value, ignoring whatever was in the column.

```yaml
transforms:
  engine:
    constant: "aws-lambda"
```

**When to use:** every row of the resource has the same engine (Lambda → `aws-lambda`).
**When not to use:** the engine genuinely varies. Use `substring_lookup` or `from_version_major` instead.

#### `default_if_empty: "<value>"`

Set the engine to a fixed value **only when the column reading is empty**. Has no effect when the engine column is non-empty.

```yaml
transforms:
  engine:
    default_if_empty: "eks"
```

**When to use:** the inventory has no engine column, so the column reading is always `""` and you want a consistent typed value.
**When not to use:** you want to override a non-empty value too. Use `constant` instead.

#### `substring_lookup`

Run rules in order. The first rule whose **every** `contains` substring is present in the lowercased engine wins. If no rule matches, the lowercased input is returned as-is.

```yaml
transforms:
  engine:
    substring_lookup:
      - contains: ["aurora", "postgres"]
        result: "aurora-postgresql"
```

- Rules are evaluated in YAML order; first match wins.
- `contains` is logical AND across the listed substrings.
- The input is already lowercased+trimmed when matching, so write rules in lowercase.

| Engine column | Rule matches? | Output |
|---|---|---|
| `AuroraPostgreSQL` | yes (`aurora` ∧ `postgres`) | `aurora-postgresql` |
| `Redis` | no rule matches | `redis` (lowercased) |

> ⚠️ **Don't include rules that can't fire.** If the resource's `native_type_pattern` already restricts rows to one engine kind (Aurora MySQL clusters never produce `AuroraPostgreSQL`), don't list a `mysql` rule on the postgres resource. Dead rules confuse the next reader.

**When to use:** the engine column has a free-form name that needs canonicalizing.
**When not to use:** the engine is a constant (`constant`), or comes from the version (`from_version_major`).

#### `from_version_major`

Look up the engine from the version's major component (the substring before the first `.`). Use this when the engine is *implicit in the version*.

```yaml
transforms:
  engine:
    from_version_major:
      majors:
        "5": "elasticsearch"
        "6": "elasticsearch"
        "7": "elasticsearch"
      default: "opensearch"
```

- The version is read **after** the version transform runs, so `strip_prefixes` results are visible to the major lookup.
- `majors` keys are exact-string matches on the major component.
- `default` is the fallback engine when the major isn't in the map (or the version is empty).

| Version | Major | Output |
|---|---|---|
| `7.10` | `7` | `elasticsearch` |
| `2.13` | `2` | `opensearch` (default) |
| `""` | — | `opensearch` (default) |

**When to use:** the engine is a function of the version (legacy/modern split).
**When not to use:** the engine is a column value (use `substring_lookup` or none).

---

## Composition rules

- A resource can declare both `transforms.version` and `transforms.engine` — they're independent.
- Within `transforms.version` you may set **at most one** of `strip_prefixes`, `extract_json_field`. Setting both is rejected at config-load time.
- Within `transforms.engine` you may set **at most one** of `constant`, `default_if_empty`, `substring_lookup`, `from_version_major`. Setting more than one is rejected at config-load time.
- The version transform always runs first, so the engine transform's `from_version_major` op sees the post-transform version.

---

## Validation guarantees

The loader validates the transforms block at startup so YAML mistakes fail fast (before any scan starts) instead of mid-scan with a confusing error. Specifically:

- Multiple version ops set at once → `set at most one of strip_prefixes, extract_json_field`.
- Multiple engine ops set at once → `set at most one of constant, default_if_empty, substring_lookup, from_version_major`.
- `extract_json_field` missing `from_column` or `field`.
- `substring_lookup` rule with empty `contains` or empty `result`.
- `from_version_major` missing `default` or `majors`.

These rules are exercised by `TestValidateConfig_Transforms` in `pkg/config/loader_test.go`.

---

## Worked examples — the canonical resources

### Aurora PostgreSQL — `substring_lookup` (engine canonicalization)

```yaml
- id: aurora-postgresql
  inventory:
    native_type_pattern: "rds/AmazonAuroraPostgreSQL/cluster"
    required_mappings:
      engine: "typeFields.kind"   # column value: "AuroraPostgreSQL"
  transforms:
    engine:
      substring_lookup:
        - contains: ["aurora", "postgres"]
          result: "aurora-postgresql"
```

### Aurora MySQL — symmetric to PostgreSQL

```yaml
- id: aurora-mysql
  inventory:
    native_type_pattern: "rds/AmazonAuroraMySQL/cluster"
    required_mappings:
      engine: "typeFields.kind"
  transforms:
    engine:
      substring_lookup:
        - contains: ["aurora", "mysql"]
          result: "aurora-mysql"
```

### EKS — `default_if_empty`

EKS reports have no engine column. The engine is implicitly `eks`.

```yaml
- id: eks
  inventory:
    required_mappings:
      resource_id: "providerUniqueId"
      version: "versionDetails.version"
    # Note: no "engine" mapping at all.
  transforms:
    engine:
      default_if_empty: "eks"
```

### OpenSearch — `strip_prefixes` + `from_version_major`

OpenSearch reports versions like `OpenSearch_2.13` and `Elasticsearch_7.10`. Strip the prefix, then derive the engine from the major.

```yaml
- id: opensearch
  inventory:
    required_mappings:
      resource_id: "externalId"
      version: "versionDetails.version"
  transforms:
    version:
      strip_prefixes: ["OpenSearch_", "Elasticsearch_"]
    engine:
      from_version_major:
        majors:
          "5": "elasticsearch"
          "6": "elasticsearch"
          "7": "elasticsearch"
        default: "opensearch"
```

### Lambda — `extract_json_field` + `constant`

Lambda runtime lives inside `graphEntity.properties` JSON; engine is always `aws-lambda`.

```yaml
- id: lambda
  inventory:
    required_mappings:
      resource_id: "externalId"
  transforms:
    version:
      extract_json_field:
        from_column: "graphEntity.properties"
        field: "runtime"
        skip_if_empty: true   # drop container-image Lambdas
    engine:
      constant: "aws-lambda"
```

### ElastiCache, RDS-MySQL, RDS-PostgreSQL — no transforms

The raw `typeFields.kind` column already produces a reasonable engine after lowercase+trim (`Redis` → `redis`, etc.). No transforms block is needed.

```yaml
- id: elasticache-redis
  inventory:
    required_mappings:
      engine: "typeFields.kind"
  # No `transforms:` — baseline lowercase+trim is enough.
```

---

## Anti-patterns

These shapes look tempting but are signals to **not** use a transform (or to add a new op instead):

- **Dead rules.** Listing a `substring_lookup` rule that the resource's `native_type_pattern` makes unreachable. (The earlier draft of `aurora-postgresql` had a `["aurora","mysql"]` rule that could never fire.) Dead rules mislead readers into thinking the resource handles cases it doesn't.
- **Catch-all substrings.** Rules with a single very common substring (`contains: ["a"]`) effectively short-circuit later rules. Make rules specific; rely on `contains` AND-semantics to disambiguate.
- **Per-row computation.** Anything that needs more than the row's columns (registry lookups, tag aggregation across rows, cross-resource correlation) belongs in the inventory layer, not transforms.
- **Transforms as feature flags.** If you find yourself writing `default_if_empty: ""` or rules with empty results to *disable* behavior, that's a design smell. Either remove the resource or fix the input column mapping.
- **Two ops "stacked" by hoping the validator fails open.** It doesn't — the loader rejects multiple sibling ops up front.

---

## Adding a new operation

If the existing ops don't fit a new resource:

1. **Sanity check.** Could the resource use one of the existing ops with different args? Often yes.
2. **Sketch the YAML you wish you could write.** Aim for one named op, single-purpose, with structured args. Resist the urge to introduce expressions, conditionals, or computed strings.
3. **Add the type** to `pkg/config/transforms.go`. Put it under the right field (`VersionTransform` or `EngineTransform`) and update the at-most-one validator.
4. **Add the applier** to `pkg/inventory/wiz/transforms.go` (or wherever the appropriate inventory parser lives — transforms aren't logically Wiz-specific, just hosted there until a second consumer arrives).
5. **Cover it in `transforms_test.go`** — happy path, empty inputs, malformed inputs, edge cases.
6. **Update the YAML** for the resource that needs it, and the table in this doc.

If the new op feels too narrow to plausibly be reused, that's a sign the reshape might belong in the inventory source as a one-off rather than as a DSL op. The DSL grows by general primitives, not bespoke shapes.
