//go:build integration
// +build integration

package override

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/block/Version-Guard/pkg/eol/endoflife"
)

// TestRepositoryOverridesStillNeeded checks every override in the
// repository manifest against the live endoflife.date API and fails when
// upstream already serves all of an override's cycles. That override is
// retirable: delete its api/<product>.json and manifest entry so
// production reads upstream data directly.
//
// Run with: go test -tags=integration ./deploy/endoflife-override
func TestRepositoryOverridesStillNeeded(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live endoflife.date check in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	results, err := checkUpstreamCoverage(ctx, endoflife.NewRealHTTPClient(), ".")
	require.NoError(t, err)
	for _, result := range results {
		t.Logf("%s: upstream_missing=%t missing_cycles=%v", result.Product, result.UpstreamMissing, result.MissingCycles)
	}
	require.Empty(t, retirableOverrides(results),
		"upstream now covers these overrides; delete api/<product>.json and the manifest entry")
}
