package examples

import (
	"context"
	"fmt"

	"github.com/block/Version-Guard/pkg/emitters"
	"github.com/block/Version-Guard/pkg/types"
)

// LoggingASREmitter is an example emitter that logs findings to stdout
// This demonstrates how to implement the ASREmitter interface
type LoggingASREmitter struct{}

// NewLoggingASREmitter creates a new logging ASR emitter
func NewLoggingASREmitter() *LoggingASREmitter {
	return &LoggingASREmitter{}
}

// Emit logs findings to stdout instead of creating actual issues
func (e *LoggingASREmitter) Emit(ctx context.Context, snapshotID string, findings []*types.Finding) (*emitters.ASRResult, error) {
	fmt.Printf("\n=== ASR Emitter (Logging Mode) ===\n")
	fmt.Printf("Snapshot ID: %s\n", snapshotID)
	fmt.Printf("Total Findings: %d\n\n", len(findings))

	created := 0
	updated := 0
	closed := 0

	for _, f := range findings {
		switch f.Status {
		case types.StatusRed, types.StatusYellow:
			// Would create or update issue
			fmt.Printf("[%s] %s (%s)\n", f.Status, f.ResourceID, f.ResourceType)
			fmt.Printf("  Message: %s\n", f.Message)
			fmt.Printf("  Recommendation: %s\n", f.Recommendation)
			fmt.Printf("  → Would create/update issue\n\n")
			created++
		case types.StatusGreen:
			// Would close issue if exists
			fmt.Printf("[%s] %s (%s)\n", f.Status, f.ResourceID, f.ResourceType)
			fmt.Printf("  → Would close issue if exists\n\n")
			closed++
		}
	}

	result := &emitters.ASRResult{
		IssuesCreated: created,
		IssuesUpdated: updated,
		IssuesClosed:  closed,
	}

	fmt.Printf("Summary: %d created/updated, %d closed\n", created, closed)
	fmt.Printf("=====================================\n\n")

	return result, nil
}

// LoggingDXEmitter is an example emitter that logs compliance summaries to stdout
// This demonstrates how to implement the DXEmitter interface
type LoggingDXEmitter struct{}

// NewLoggingDXEmitter creates a new logging DX emitter
func NewLoggingDXEmitter() *LoggingDXEmitter {
	return &LoggingDXEmitter{}
}

// Emit logs compliance summary to stdout instead of pushing to DX Scorecards
func (e *LoggingDXEmitter) Emit(ctx context.Context, snapshotID string, summary *types.SnapshotSummary) (*emitters.DXResult, error) {
	fmt.Printf("\n=== DX Scorecard Emitter (Logging Mode) ===\n")
	fmt.Printf("Snapshot ID: %s\n\n", snapshotID)

	fmt.Printf("Overall Compliance:\n")
	fmt.Printf("  Total Resources: %d\n", summary.TotalResources)
	fmt.Printf("  Compliance: %.1f%%\n\n", summary.CompliancePercentage)

	fmt.Printf("Status Breakdown:\n")
	fmt.Printf("  🔴 Red:     %d (%.1f%%)\n", summary.RedCount, float64(summary.RedCount)*100/float64(summary.TotalResources))
	fmt.Printf("  🟡 Yellow:  %d (%.1f%%)\n", summary.YellowCount, float64(summary.YellowCount)*100/float64(summary.TotalResources))
	fmt.Printf("  🟢 Green:   %d (%.1f%%)\n", summary.GreenCount, float64(summary.GreenCount)*100/float64(summary.TotalResources))
	fmt.Printf("  ⚪ Unknown: %d (%.1f%%)\n\n", summary.UnknownCount, float64(summary.UnknownCount)*100/float64(summary.TotalResources))

	servicesUpdated := len(summary.ByService)
	fmt.Printf("Would update %d services in DX Scorecard\n", servicesUpdated)
	fmt.Printf("==========================================\n\n")

	return &emitters.DXResult{
		ServicesUpdated: servicesUpdated,
	}, nil
}
