package emitters

import (
	"context"

	"github.com/block/Version-Guard/pkg/types"
)

// ASREmitter emits findings to AppSecReporter (SCC issue tracking)
type ASREmitter interface {
	// Emit creates or updates ASR issues for the given findings
	Emit(ctx context.Context, snapshotID string, findings []*types.Finding) (*ASRResult, error)
}

// ASRResult contains the result of emitting to ASR
type ASRResult struct {
	IssuesCreated int
	IssuesUpdated int
	IssuesClosed  int
}

// DXEmitter pushes compliance data to DX Scorecards
type DXEmitter interface {
	// Emit pushes summary statistics to DX Scorecards
	Emit(ctx context.Context, snapshotID string, summary *types.SnapshotSummary) (*DXResult, error)
}

// DXResult contains the result of emitting to DX Scorecards
type DXResult struct {
	ServicesUpdated int
}
