package detector

import (
	"context"

	"github.com/block/Version-Guard/pkg/types"
)

// Detector defines the interface for version drift detection
type Detector interface {
	// Detect scans resources and returns findings
	Detect(ctx context.Context) ([]*types.Finding, error)

	// ResourceType returns the type of resource this detector handles
	ResourceType() types.ResourceType

	// Name returns the name of this detector
	Name() string
}
