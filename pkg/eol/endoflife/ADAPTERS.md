# Schema Adapters — and why EKS needs its own

`endoflife.date` is the single upstream source for every EOL provider in
Version Guard, but it is **not** a uniform schema. Most products use the
"standard" cycle shape; a handful use product-specific semantics where
the same field name means a different thing. The `SchemaAdapter`
interface in [adapters.go](./adapters.go) is the seam where those
deviations are absorbed so the rest of Version Guard sees a single,
canonical `types.VersionLifecycle`.

This doc exists because the EKS deviation is the kind of thing that
will silently mis-classify clusters in production if you wire it up the
"obvious" way.

---

## The standard schema (what most products look like)

Example cycle for `amazon-aurora-postgresql`:

```json
{
  "cycle": "17",
  "releaseDate": "2025-02-20",
  "support": "2030-02-28",
  "eol": "2030-02-28",
  "extendedSupport": "2033-02-28"
}
```

`StandardSchemaAdapter` maps these as you would expect:

| endoflife.date field | `VersionLifecycle` field      | meaning                                  |
| -------------------- | ----------------------------- | ---------------------------------------- |
| `support`            | `DeprecationDate`             | end of standard support                  |
| `eol`                | `EOLDate`                     | true end of life — version stops working |
| `extendedSupport`    | `ExtendedSupportEnd`          | last day AWS will sell extended support  |

A version past `eol` is `IsEOL=true`, classified RED by the policy
layer. A version past `support` but before `eol` is in extended support,
classified YELLOW. Simple.

---

## The EKS gotcha — three deviations from the standard

EKS does not match the standard schema in three concrete ways:

1. The same field name (`cycle.eol`) means a different thing on EKS
   than on standard products.
2. EKS removes a concept that standard products always have (true EOL).
3. The adapter compensates for (1) and (2) with a routing that itself
   loses information — output field name and source field name do not
   mean the same thing.

Each deviation, in turn:

### Deviation 1 — `cycle.eol` is **not** the true EOL

For `amazon-eks`, the `eol` field is the day **standard support ends**,
not the day the version stops working. Compare:

```json
// amazon-eks cycle 1.31 (live data)
{
  "cycle": "1.31",
  "eol": "2025-11-26",                 // ← standard-support end, NOT true EOL
  "extendedSupport": "2026-11-26"      // ← extended-support end
}
```

If you ran cycle 1.31 through `StandardSchemaAdapter`, today
(2026-04-28) it would be flagged `IsEOL=true` — implying the cluster
has stopped working. It hasn't. The cluster is in extended support and
will be supported by AWS until 2026-11-26. `EKSSchemaAdapter` avoids
that specific misclassification by hard-setting `lifecycle.EOLDate =
nil` (see Deviation 2) and routing `cycle.eol` into
`lifecycle.ExtendedSupportEnd` instead.

(Even after this routing the in-extended-support classification is
still imperfect — see Deviation 3 — but no version is ever reported as
past true EOL.)

### Deviation 2 — EKS has no true EOL

EKS clusters never stop working. Once you are past extended support AWS
stops issuing patches, but the control plane keeps running on the old
version indefinitely. There is no equivalent of the standard `eol`
field for EKS, and the adapter encodes that by hard-setting
`lifecycle.EOLDate = nil` regardless of input. Any classifier rule
keyed on `EOLDate` is therefore inert for EKS — the policy reads
`ExtendedSupportEnd` instead.

### Deviation 3 — `cycle.eol` (standard-support-end) is mapped onto `ExtendedSupportEnd`; `cycle.extendedSupport` is ignored

The adapter routes `cycle.eol` (which marks **standard-support-end** —
see Deviation 1) into the output field `lifecycle.ExtendedSupportEnd`,
and ignores `cycle.extendedSupport` entirely. This is the part of the
adapter most likely to mislead a reader of the code: input field name
and output field name **do not** mean the same thing.

In effect, the adapter pretends standard-support-end is
extended-support-end. For cycle 1.31:

| Source field            | Source value | Real meaning             | Mapped to                    | Effect                                                              |
| ----------------------- | ------------ | ------------------------ | ---------------------------- | ------------------------------------------------------------------- |
| `cycle.eol`             | `2025-11-26` | standard-support-end     | `lifecycle.ExtendedSupportEnd` | adapter treats this date as the policy threshold for "past extended support" |
| `cycle.extendedSupport` | `2026-11-26` | extended-support-end     | (ignored)                    | the real extended-support window is invisible to the policy layer    |

Historical reason: when the adapter was written, endoflife.date
returned `extendedSupport` as a boolean flag, so `cycle.eol` was the
only available date signal. Live data now returns a real date in
`extendedSupport`, but the adapter has not been updated.

**Consequence:** an EKS version that is genuinely in extended support
today is classified as past-extended-support (RED) by the policy
layer. This is a known coarsening — it errs toward urging upgrades,
which matches the intended product behavior, but it is a real
semantic gap to be aware of when reading EKS findings, and a
candidate for a follow-up fix.

---

## What the adapter actually outputs

For `amazon-eks` cycle 1.31 (`eol: 2025-11-26`,
`extendedSupport: 2026-11-26`), evaluated on 2026-04-28 — a date that
is genuinely inside the AWS-defined extended-support window
(2025-11-26 → 2026-11-26):

| Field                          | Value        | Source / note                                                                 |
| ------------------------------ | ------------ | ----------------------------------------------------------------------------- |
| `EOLDate`                      | `nil`        | always for EKS (Deviation 2)                                                  |
| `DeprecationDate`              | `nil`        | EKS cycles have no `cycle.support` field                                      |
| `ExtendedSupportEnd`           | `2025-11-26` | sourced from **`cycle.eol`** (standard-support-end) — *not* `cycle.extendedSupport`; see Deviation 3 |
| `IsExtendedSupport`            | `false`*     | branch is unreachable in practice — see footnote                              |
| `IsSupported` / `IsDeprecated` | `false` / `true` | classified RED, because `now` (2026-04-28) is past `ExtendedSupportEnd` (2025-11-26) — even though AWS is still in real extended support until 2026-11-26 |

\* The adapter reports `IsExtendedSupport=true` only while
`now` is between `cycle.support` and `cycle.eol` — and EKS cycles have
no `cycle.support` field, so the "in extended support" branch is
unreachable in practice. Combined with Deviation 3, the policy layer
never sees an EKS version as YELLOW; it's GREEN until it crosses
`cycle.eol` (standard-support-end), then RED.

---

## Picking the right adapter

The adapter is selected per-resource via YAML — `eol.schema` on the
resource entry, validated by the config loader at startup:

```yaml
- id: eks
  eol:
    provider: endoflife-date
    product: amazon-eks
    schema: eks_adapter        # ← the EKS gotcha lives here
```

```yaml
- id: aurora-postgresql
  eol:
    provider: endoflife-date
    product: amazon-aurora-postgresql
    schema: standard           # ← the default for almost everything
```

Empty `schema` defaults to `standard`. Adding a new schema means
implementing `SchemaAdapter`, registering it in `SchemaAdapters` in
[adapters.go](./adapters.go), and naming it from YAML — no Go change in
the resource detector, the activities, or the policy.

---

## Adding a new adapter — the rule of thumb

If a new product cycle's fields have different semantics from the
standard ones, write an adapter. Symptoms that indicate you need one:

- A field's name suggests one thing but the dates encode another (the
  EKS `eol` case above).
- A field is a boolean where the standard schema expects a date, or
  vice versa.
- The product is missing a concept the standard schema relies on
  (EKS having no true EOL).
- Comparing the live JSON cycle to the standard layout shows a
  field that should never be parsed as a date, or a date that should
  never be treated as the true EOL.

If a new product matches the standard semantics, do not write an
adapter — use `standard` and move on. The point of this seam is to keep
deviations explicit and small, not to encode every product separately.

---

## When in doubt, fetch the live cycle

```sh
curl -s https://endoflife.date/api/amazon-eks.json | jq '.[0]'
curl -s https://endoflife.date/api/amazon-aurora-postgresql.json | jq '.[0]'
```

Two cycles, side by side, will show you in seconds whether you are
looking at a standard-schema product or another EKS-style gotcha. If
the field shapes match the standard table at the top of this doc, ship
it as `schema: standard`. If they don't, write an adapter and add a row
to this doc.
