package wiz

import (
	"context"
	"log"

	"github.com/pkg/errors"

	"github.com/block/Version-Guard/pkg/types"
)

// rowFilterFunc decides whether a CSV row should be processed
// Returns true if the row should be parsed, false to skip it
type rowFilterFunc func(row []string) bool

// rowParserFunc parses a CSV row into a Resource
// Returns nil resource to skip, non-nil to include in results
type rowParserFunc func(ctx context.Context, row []string) (*types.Resource, error)

// parseWizReport is a shared helper that implements the common CSV-row-iteration pattern
// used by Aurora, EKS, and ElastiCache inventory sources.
//
// This eliminates ~90 lines of duplicated code across the three sources and ensures
// consistent error handling and processing logic.
//
// Parameters:
//   - ctx: Context for the operation
//   - client: Wiz client for fetching report data
//   - reportID: The Wiz report ID to fetch
//   - minColumns: Minimum number of columns required in each row
//   - filterRow: Function to filter rows (e.g., check nativeType)
//   - parseRow: Function to parse a valid row into a Resource
//
// Returns:
//   - List of successfully parsed resources
//   - Error if report fetching fails (not if individual rows fail to parse)
func parseWizReport(
	ctx context.Context,
	client *Client,
	reportID string,
	minColumns int,
	filterRow rowFilterFunc,
	parseRow rowParserFunc,
) ([]*types.Resource, error) {
	// Fetch report data
	rows, err := client.GetReportData(ctx, reportID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch Wiz report data")
	}

	if len(rows) < 2 {
		// Empty report (only header row)
		return []*types.Resource{}, nil
	}

	// Skip header row, parse data rows
	var resources []*types.Resource
	for i, row := range rows[1:] {
		// Ensure row has minimum required columns
		if len(row) < minColumns {
			// Skip malformed rows
			continue
		}

		// Apply resource type filter
		if !filterRow(row) {
			continue
		}

		// Parse the row
		resource, err := parseRow(ctx, row)
		if err != nil {
			// Log error but continue processing other rows
			// TODO: wire through proper structured logger (e.g., *slog.Logger)
			log.Printf("WARN: row %d: failed to parse resource: %v", i+1, err)
			continue
		}

		if resource != nil {
			resources = append(resources, resource)
		}
	}

	return resources, nil
}
