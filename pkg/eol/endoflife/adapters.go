package endoflife

import (
	"time"

	"github.com/pkg/errors"

	"github.com/block/Version-Guard/pkg/types"
)

// SchemaAdapter adapts endoflife.date ProductCycle to VersionLifecycle
// Some products use non-standard field semantics and need custom adapters
type SchemaAdapter interface {
	AdaptCycle(cycle *ProductCycle) (*types.VersionLifecycle, error)
}

// StandardSchemaAdapter handles products with standard endoflife.date schema
// Standard semantics:
//   - cycle.support → DeprecationDate (end of standard support)
//   - cycle.eol → EOLDate (true end of life)
//   - cycle.extendedSupport → ExtendedSupportEnd
type StandardSchemaAdapter struct{}

// AdaptCycle converts a ProductCycle to VersionLifecycle using standard semantics
func (a *StandardSchemaAdapter) AdaptCycle(cycle *ProductCycle) (*types.VersionLifecycle, error) {
	lifecycle := &types.VersionLifecycle{
		Version:   cycle.Cycle,
		Engine:    "", // Set by caller
		Source:    providerName,
		FetchedAt: time.Now(),
	}

	// Parse release date
	if cycle.ReleaseDate != "" {
		if releaseDate, err := parseDate(cycle.ReleaseDate); err == nil {
			lifecycle.ReleaseDate = &releaseDate
		}
	}

	// Parse EOL date (STANDARD semantics: true end of life)
	var eolDate *time.Time
	if dateStr := anyToDateString(cycle.EOL); dateStr != "" {
		if parsed, err := parseDate(dateStr); err == nil {
			eolDate = &parsed
			lifecycle.EOLDate = eolDate
		}
	}

	// Parse support date (STANDARD semantics: end of standard support)
	var supportDate *time.Time
	if dateStr := anyToDateString(cycle.Support); dateStr != "" {
		if parsed, err := parseDate(dateStr); err == nil {
			supportDate = &parsed
			lifecycle.DeprecationDate = supportDate
		}
	}

	// Parse extended support
	var extendedSupportDate *time.Time
	if cycle.ExtendedSupport != nil {
		switch v := cycle.ExtendedSupport.(type) {
		case string:
			if v != "" && v != falseBool {
				if parsed, err := parseDate(v); err == nil {
					extendedSupportDate = &parsed
					lifecycle.ExtendedSupportEnd = extendedSupportDate
				}
			}
		case bool:
			// If boolean true, use EOL date as extended support end
			if v && eolDate != nil {
				extendedSupportDate = eolDate
				lifecycle.ExtendedSupportEnd = eolDate
			}
		}
	}

	// Determine lifecycle status based on dates
	now := time.Now()

	// If we have an EOL date and we're past it, mark as EOL
	if eolDate != nil && now.After(*eolDate) {
		lifecycle.IsEOL = true
		lifecycle.IsSupported = false
		lifecycle.IsDeprecated = true
		return lifecycle, nil
	}

	// If we have extended support end and we're past standard support
	if extendedSupportDate != nil && supportDate != nil && now.After(*supportDate) {
		if now.Before(*extendedSupportDate) {
			// In extended support window
			lifecycle.IsSupported = true
			lifecycle.IsExtendedSupport = true
			lifecycle.IsDeprecated = true
		} else {
			// Past extended support
			lifecycle.IsEOL = true
			lifecycle.IsSupported = false
			lifecycle.IsDeprecated = true
		}
		return lifecycle, nil
	}

	// If we're past support date but no extended support info
	if supportDate != nil && now.After(*supportDate) {
		lifecycle.IsDeprecated = true
		lifecycle.IsSupported = false
		// If we have EOL date, use it; otherwise mark as deprecated but not EOL
		if eolDate != nil && now.Before(*eolDate) {
			lifecycle.IsEOL = false
		}
		return lifecycle, nil
	}

	// Still in standard support
	lifecycle.IsSupported = true
	lifecycle.IsDeprecated = false
	lifecycle.IsEOL = false

	return lifecycle, nil
}

// EKSSchemaAdapter handles amazon-eks product with NON-STANDARD schema
// EKS semantics (DIFFERENT from standard):
//   - cycle.support → DeprecationDate (end of standard support) ✅ Same
//   - cycle.eol → ExtendedSupportEnd (NOT true EOL!) ⚠️ DIFFERENT
//   - cycle.extendedSupport → boolean flag (NOT a date) ⚠️ DIFFERENT
//   - EKS has NO true EOL (clusters keep running forever)
type EKSSchemaAdapter struct{}

// AdaptCycle converts EKS ProductCycle to VersionLifecycle using EKS-specific semantics
func (a *EKSSchemaAdapter) AdaptCycle(cycle *ProductCycle) (*types.VersionLifecycle, error) {
	lifecycle := &types.VersionLifecycle{
		Version:   cycle.Cycle,
		Engine:    "eks",
		Source:    providerName,
		FetchedAt: time.Now(),
	}

	// Parse release date (standard)
	if cycle.ReleaseDate != "" {
		if releaseDate, err := parseDate(cycle.ReleaseDate); err == nil {
			lifecycle.ReleaseDate = &releaseDate
		}
	}

	// Parse standard support end (standard)
	var supportDate *time.Time
	if dateStr := anyToDateString(cycle.Support); dateStr != "" {
		if parsed, err := parseDate(dateStr); err == nil {
			supportDate = &parsed
			lifecycle.DeprecationDate = supportDate
		}
	}

	// ⚠️ NON-STANDARD: cycle.EOL → ExtendedSupportEnd (NOT EOLDate!)
	var extendedSupportEnd *time.Time
	if dateStr := anyToDateString(cycle.EOL); dateStr != "" {
		if parsed, err := parseDate(dateStr); err == nil {
			extendedSupportEnd = &parsed
			lifecycle.ExtendedSupportEnd = &parsed
		}
	}

	// EKS has NO true EOL (clusters keep running forever)
	lifecycle.EOLDate = nil

	// Determine lifecycle status
	now := time.Now()

	if extendedSupportEnd != nil && now.After(*extendedSupportEnd) {
		// Past extended support
		lifecycle.IsEOL = false // NOT true EOL, just no AWS support
		lifecycle.IsSupported = false
		lifecycle.IsDeprecated = true
		lifecycle.IsExtendedSupport = false
	} else if supportDate != nil && now.After(*supportDate) {
		// In extended support window
		lifecycle.IsSupported = true
		lifecycle.IsExtendedSupport = true
		lifecycle.IsDeprecated = true
		lifecycle.IsEOL = false
	} else {
		// Still in standard support
		lifecycle.IsSupported = true
		lifecycle.IsDeprecated = false
		lifecycle.IsEOL = false
		lifecycle.IsExtendedSupport = false
	}

	return lifecycle, nil
}

// SchemaAdapters is a registry of available schema adapters
var SchemaAdapters = map[string]SchemaAdapter{
	"standard":    &StandardSchemaAdapter{},
	"eks_adapter": &EKSSchemaAdapter{},
}

// GetSchemaAdapter returns the appropriate schema adapter for a product
func GetSchemaAdapter(schemaName string) (SchemaAdapter, error) {
	adapter, ok := SchemaAdapters[schemaName]
	if !ok {
		return nil, errors.Errorf("unknown schema adapter: %s", schemaName)
	}
	return adapter, nil
}
