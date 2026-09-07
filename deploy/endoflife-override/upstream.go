package override

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/pkg/errors"

	"github.com/block/Version-Guard/pkg/eol/endoflife"
)

// upstreamCoverage records how much of a local override the upstream
// endoflife.date API already serves. An override exists to fill a gap
// upstream; once upstream serves every cycle the override carries, the
// override is retirable and should be deleted so production reads the
// authoritative data.
type upstreamCoverage struct {
	Product string
	// MissingCycles are cycles present in the local override but absent
	// upstream, in local file order. Empty when upstream covers everything.
	MissingCycles []string
	// UpstreamMissing is true when upstream returns 404 for the product.
	UpstreamMissing bool
}

// Retirable reports whether upstream fully covers the override.
func (c upstreamCoverage) Retirable() bool {
	return !c.UpstreamMissing && len(c.MissingCycles) == 0
}

// checkUpstreamCoverage compares every override in root/manifest.json
// against the cycles the client returns for the same product. Products
// are checked in manifest order. A 404 upstream is a coverage result,
// not an error; any other fetch or decode failure aborts the check.
func checkUpstreamCoverage(ctx context.Context, client endoflife.Client, root string) ([]upstreamCoverage, error) {
	m, err := readManifest(filepath.Join(root, "manifest.json"))
	if err != nil {
		return nil, err
	}

	results := make([]upstreamCoverage, 0, len(m.Overrides))
	for _, override := range m.Overrides {
		localCycles, err := readCycles(filepath.Join(root, filepath.FromSlash(override.Path)))
		if err != nil {
			return nil, errors.Wrapf(err, "override %q", override.Product)
		}

		coverage := upstreamCoverage{Product: override.Product}
		upstream, err := client.GetProductCycles(ctx, override.Product)
		switch {
		case errors.Is(err, endoflife.ErrProductNotFound):
			coverage.UpstreamMissing = true
		case err != nil:
			return nil, errors.Wrapf(err, "fetch upstream cycles for %q", override.Product)
		default:
			coverage.MissingCycles = missingCycles(localCycles, upstream.Cycles)
		}
		results = append(results, coverage)
	}
	return results, nil
}

// retirableOverrides returns the products whose override upstream now
// fully covers, sorted for stable output.
func retirableOverrides(results []upstreamCoverage) []string {
	var products []string
	for _, result := range results {
		if result.Retirable() {
			products = append(products, result.Product)
		}
	}
	sort.Strings(products)
	return products
}

func missingCycles(local, upstream []*endoflife.ProductCycle) []string {
	present := make(map[string]struct{}, len(upstream))
	for _, cycle := range upstream {
		if cycle != nil {
			present[cycle.Cycle] = struct{}{}
		}
	}
	missing := []string{}
	for _, cycle := range local {
		if cycle == nil {
			continue
		}
		if _, ok := present[cycle.Cycle]; !ok {
			missing = append(missing, cycle.Cycle)
		}
	}
	return missing
}

func readCycles(path string) ([]*endoflife.ProductCycle, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read API file: %w", err)
	}
	var cycles []*endoflife.ProductCycle
	if err := json.Unmarshal(data, &cycles); err != nil {
		return nil, fmt.Errorf("decode API file %q: %w", path, err)
	}
	return cycles, nil
}
