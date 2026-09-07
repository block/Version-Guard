package override

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/block/Version-Guard/pkg/eol/endoflife"
)

func cycles(ids ...string) []*endoflife.ProductCycle {
	out := make([]*endoflife.ProductCycle, 0, len(ids))
	for _, id := range ids {
		out = append(out, &endoflife.ProductCycle{Cycle: id})
	}
	return out
}

func TestCheckUpstreamCoverage(t *testing.T) {
	tests := []struct {
		name          string
		upstream      map[string][]*endoflife.ProductCycle
		notFound      map[string]bool
		wantCoverage  []upstreamCoverage
		wantRetirable []string
	}{
		{
			name: "upstream covers every override",
			upstream: map[string][]*endoflife.ProductCycle{
				"sample-product":     cycles("8.4", "3", "2", "1"),
				"sample-product-two": cycles("3.5", "3.3", "3.1"),
			},
			wantCoverage: []upstreamCoverage{
				{Product: "sample-product", MissingCycles: []string{}},
				{Product: "sample-product-two", MissingCycles: []string{}},
			},
			wantRetirable: []string{"sample-product", "sample-product-two"},
		},
		{
			name: "upstream still missing cycles",
			upstream: map[string][]*endoflife.ProductCycle{
				"sample-product":     cycles("3", "2"),
				"sample-product-two": cycles("3.1", "2.19"),
			},
			wantCoverage: []upstreamCoverage{
				{Product: "sample-product", MissingCycles: []string{}},
				{Product: "sample-product-two", MissingCycles: []string{"3.5", "3.3"}},
			},
			wantRetirable: []string{"sample-product"},
		},
		{
			name:     "upstream product not found is still needed",
			upstream: map[string][]*endoflife.ProductCycle{"sample-product-two": cycles("3.5", "3.3")},
			notFound: map[string]bool{"sample-product": true},
			wantCoverage: []upstreamCoverage{
				{Product: "sample-product", UpstreamMissing: true},
				{Product: "sample-product-two", MissingCycles: []string{}},
			},
			wantRetirable: []string{"sample-product-two"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &endoflife.MockClient{GetProductCyclesFunc: func(_ context.Context, product string) (endoflife.ProductCyclesResult, error) {
				if tt.notFound[product] {
					return endoflife.ProductCyclesResult{}, endoflife.ErrProductNotFound
				}
				return endoflife.ProductCyclesResult{Cycles: tt.upstream[product]}, nil
			}}

			results, err := checkUpstreamCoverage(context.Background(), client, copyFixture(t))
			require.NoError(t, err)
			require.Equal(t, tt.wantCoverage, results)
			require.Equal(t, tt.wantRetirable, retirableOverrides(results))
		})
	}
}

func TestCheckUpstreamCoverageFetchErrorAborts(t *testing.T) {
	client := &endoflife.MockClient{GetProductCyclesFunc: func(context.Context, string) (endoflife.ProductCyclesResult, error) {
		return endoflife.ProductCyclesResult{}, errors.New("upstream unavailable")
	}}

	_, err := checkUpstreamCoverage(context.Background(), client, copyFixture(t))
	require.ErrorContains(t, err, "upstream unavailable")
}

func TestCheckUpstreamCoverageNoOverrides(t *testing.T) {
	client := &endoflife.MockClient{GetProductCyclesFunc: func(context.Context, string) (endoflife.ProductCyclesResult, error) {
		t.Fatal("client must not be called when the manifest has no overrides")
		return endoflife.ProductCyclesResult{}, nil
	}}

	results, err := checkUpstreamCoverage(context.Background(), client, ".")
	require.NoError(t, err)
	require.Empty(t, results)
	require.Empty(t, retirableOverrides(results))
}
