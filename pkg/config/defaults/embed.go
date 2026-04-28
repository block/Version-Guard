// Package defaults exposes the canonical resources.yaml that Version
// Guard ships with, embedded into the binary at build time.
//
// At runtime the loader uses ResourcesYAML when no CONFIG_PATH override
// is supplied, so the binary is self-contained and works out of the
// box without any sidecar config file. Operators who want a different
// resource set, mappings, or EOL providers point CONFIG_PATH at their
// own YAML file and the embedded default is ignored.
package defaults

import _ "embed"

// ResourcesYAML is the canonical resources configuration shipped with
// the binary. Its content is the verbatim bytes of resources.yaml in
// this package directory; edit that file to change the default
// resource catalog.
//
//go:embed resources.yaml
var ResourcesYAML []byte
