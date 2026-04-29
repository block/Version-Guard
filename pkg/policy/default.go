package policy

import (
	"fmt"
	"strings"
	"time"

	"github.com/block/Version-Guard/pkg/types"
)

// DefaultPolicy implements the standard version compliance classification rules
type DefaultPolicy struct {
	// EOLWarningDays is the number of days before EOL to start showing YELLOW warnings
	EOLWarningDays int

	// WarnExtendedSupport indicates whether to warn about extended support versions
	WarnExtendedSupport bool
}

// NewDefaultPolicy creates a new DefaultPolicy with standard settings
func NewDefaultPolicy() *DefaultPolicy {
	return &DefaultPolicy{
		EOLWarningDays:      90,
		WarnExtendedSupport: true,
	}
}

// Name returns the name of this policy
func (p *DefaultPolicy) Name() string {
	return "DefaultVersionPolicy"
}

// Classify determines the compliance status based on version lifecycle
//
// Classification Rules:
// - RED: Past EOL, deprecated, or extended support expired
// - YELLOW: In extended support (costly), or approaching EOL (< 90 days)
// - GREEN: Current supported version
// - UNKNOWN: Version not found in EOL database
func (p *DefaultPolicy) Classify(resource *types.Resource, lifecycle *types.VersionLifecycle) types.Status {
	// If lifecycle data is empty or version doesn't match, return UNKNOWN.
	// endoflife.date uses major.minor cycles (e.g., "8.0") while resources
	// have full versions (e.g., "8.0.35"), so we use prefix matching.
	if lifecycle.Version == "" || !versionMatches(lifecycle.Version, resource.CurrentVersion) {
		return types.StatusUnknown
	}

	// Check for RED status conditions
	if p.isRedStatus(lifecycle) {
		return types.StatusRed
	}

	// Check for YELLOW status conditions
	if p.isYellowStatus(lifecycle) {
		return types.StatusYellow
	}

	// GREEN: Currently supported
	if lifecycle.IsSupported {
		return types.StatusGreen
	}

	// Default to UNKNOWN if we can't determine status
	return types.StatusUnknown
}

// isRedStatus checks if the lifecycle indicates a RED status
func (p *DefaultPolicy) isRedStatus(lifecycle *types.VersionLifecycle) bool {
	// Past End-of-Life
	if lifecycle.IsEOL {
		return true
	}

	// Deprecated (but not if still in extended support)
	if lifecycle.IsDeprecated && !lifecycle.IsExtendedSupport {
		return true
	}

	// Extended support has ended
	if lifecycle.ExtendedSupportEnd != nil && time.Now().After(*lifecycle.ExtendedSupportEnd) {
		return true
	}

	return false
}

// isYellowStatus checks if the lifecycle indicates a YELLOW status
func (p *DefaultPolicy) isYellowStatus(lifecycle *types.VersionLifecycle) bool {
	// In extended support (higher cost)
	if p.WarnExtendedSupport && lifecycle.IsExtendedSupport {
		return true
	}

	// Approaching EOL (within warning window)
	if lifecycle.EOLDate != nil {
		daysUntilEOL := int(time.Until(*lifecycle.EOLDate).Hours() / 24)
		if daysUntilEOL > 0 && daysUntilEOL <= p.EOLWarningDays {
			return true
		}
	}

	// Approaching deprecation
	if lifecycle.DeprecationDate != nil {
		daysUntilDeprecation := int(time.Until(*lifecycle.DeprecationDate).Hours() / 24)
		if daysUntilDeprecation > 0 && daysUntilDeprecation <= p.EOLWarningDays {
			return true
		}
	}

	return false
}

// GetMessage generates a human-readable message describing the status
func (p *DefaultPolicy) GetMessage(resource *types.Resource, lifecycle *types.VersionLifecycle, status types.Status) string {
	switch status {
	case types.StatusRed:
		return p.getRedMessage(resource, lifecycle)
	case types.StatusYellow:
		return p.getYellowMessage(resource, lifecycle)
	case types.StatusGreen:
		return p.getGreenMessage(resource, lifecycle)
	case types.StatusUnknown:
		return p.getUnknownMessage(resource, lifecycle)
	default:
		return fmt.Sprintf("Unknown status for %s version %s", resource.Engine, resource.CurrentVersion)
	}
}

func (p *DefaultPolicy) getRedMessage(resource *types.Resource, lifecycle *types.VersionLifecycle) string {
	if lifecycle.IsEOL && lifecycle.EOLDate != nil {
		return fmt.Sprintf("Version %s of %s is past End-of-Life (EOL since %s)",
			resource.CurrentVersion,
			resource.Engine,
			lifecycle.EOLDate.Format("Jan 2006"))
	}

	if lifecycle.IsDeprecated && lifecycle.DeprecationDate != nil {
		return fmt.Sprintf("Version %s of %s is deprecated (since %s)",
			resource.CurrentVersion,
			resource.Engine,
			lifecycle.DeprecationDate.Format("Jan 2006"))
	}

	if lifecycle.ExtendedSupportEnd != nil && time.Now().After(*lifecycle.ExtendedSupportEnd) {
		return fmt.Sprintf("Extended support for %s version %s has ended (ended %s)",
			resource.Engine,
			resource.CurrentVersion,
			lifecycle.ExtendedSupportEnd.Format("Jan 2006"))
	}

	return fmt.Sprintf("Version %s of %s requires immediate attention", resource.CurrentVersion, resource.Engine)
}

func (p *DefaultPolicy) getYellowMessage(resource *types.Resource, lifecycle *types.VersionLifecycle) string {
	if lifecycle.IsExtendedSupport {
		return fmt.Sprintf("Version %s of %s is in extended support (6x standard cost)",
			resource.CurrentVersion,
			resource.Engine)
	}

	if lifecycle.EOLDate != nil {
		daysUntilEOL := int(time.Until(*lifecycle.EOLDate).Hours() / 24)
		if daysUntilEOL > 0 && daysUntilEOL <= p.EOLWarningDays {
			return fmt.Sprintf("Version %s of %s will reach End-of-Life in %d days (on %s)",
				resource.CurrentVersion,
				resource.Engine,
				daysUntilEOL,
				lifecycle.EOLDate.Format("Jan 2, 2006"))
		}
	}

	if lifecycle.DeprecationDate != nil {
		daysUntilDeprecation := int(time.Until(*lifecycle.DeprecationDate).Hours() / 24)
		if daysUntilDeprecation > 0 && daysUntilDeprecation <= p.EOLWarningDays {
			return fmt.Sprintf("Version %s of %s will be deprecated in %d days (on %s)",
				resource.CurrentVersion,
				resource.Engine,
				daysUntilDeprecation,
				lifecycle.DeprecationDate.Format("Jan 2, 2006"))
		}
	}

	return fmt.Sprintf("Version %s of %s should be upgraded soon", resource.CurrentVersion, resource.Engine)
}

//nolint:unparam // lifecycle may be used in future enhancements
func (p *DefaultPolicy) getGreenMessage(resource *types.Resource, lifecycle *types.VersionLifecycle) string {
	return fmt.Sprintf("Version %s of %s is currently supported", resource.CurrentVersion, resource.Engine)
}

func (p *DefaultPolicy) getUnknownMessage(resource *types.Resource, lifecycle *types.VersionLifecycle) string {
	if lifecycle.Version == "" {
		return fmt.Sprintf("No lifecycle data available for %s version %s", resource.Engine, resource.CurrentVersion)
	}

	return fmt.Sprintf("Unable to determine support status for %s version %s", resource.Engine, resource.CurrentVersion)
}

// GetRecommendation generates a recommendation for addressing the issue
func (p *DefaultPolicy) GetRecommendation(resource *types.Resource, lifecycle *types.VersionLifecycle, status types.Status) string {
	switch status {
	case types.StatusRed:
		return p.getRedRecommendation(resource, lifecycle)
	case types.StatusYellow:
		return p.getYellowRecommendation(resource, lifecycle)
	case types.StatusGreen:
		return "No action required"
	case types.StatusUnknown:
		return "Verify version and check EOL database"
	default:
		return "Unable to provide recommendation"
	}
}

// usableUpgradeTarget returns candidate as the suggested upgrade
// target if it's both non-empty AND different from the resource's
// current cycle. Recommending the same cycle the resource is already
// on (which can happen on the YELLOW approaching-EOL path when the
// user is already on the newest supported cycle) would produce a
// confusing "Upgrade to X" message that's effectively a no-op, so we
// fall through to the generic wording instead.
//
// Callers pass the right candidate for their context:
//
//   - getRedRecommendation and the YELLOW approaching-EOL branch use
//     lifecycle.RecommendedVersion (extended-support fallback allowed —
//     a target in extended support is still better than no target).
//   - The YELLOW IsExtendedSupport branch uses
//     lifecycle.RecommendedNonExtendedVersion. Suggesting another
//     extended-support cycle there would falsely promise the upgrade
//     "avoids extended support costs" when it doesn't.
func usableUpgradeTarget(resource *types.Resource, lifecycle *types.VersionLifecycle, candidate string) string {
	if candidate == "" {
		return ""
	}
	if candidate == lifecycle.Version || candidate == resource.CurrentVersion {
		return ""
	}
	return candidate
}

func (p *DefaultPolicy) getRedRecommendation(resource *types.Resource, lifecycle *types.VersionLifecycle) string {
	// Past EOL — any supported cycle restores support, so prefer the
	// general RecommendedVersion (extended-support fallback included).
	if rec := usableUpgradeTarget(resource, lifecycle, lifecycle.RecommendedVersion); rec != "" {
		return fmt.Sprintf("Upgrade to %s %s immediately to restore support",
			resource.Engine, rec)
	}

	return fmt.Sprintf("Upgrade to the latest supported version of %s immediately", resource.Engine)
}

func (p *DefaultPolicy) getYellowRecommendation(resource *types.Resource, lifecycle *types.VersionLifecycle) string {
	if lifecycle.IsExtendedSupport {
		// "Avoid extended support costs" requires a target that is
		// itself NOT in extended support. RecommendedNonExtendedVersion
		// is empty when every supported cycle for this product is
		// already in extended support — fall back to the neutral
		// wording rather than over-promising cost relief.
		if rec := usableUpgradeTarget(resource, lifecycle, lifecycle.RecommendedNonExtendedVersion); rec != "" {
			return fmt.Sprintf("Upgrade to %s %s to avoid extended support costs",
				resource.Engine, rec)
		}
		return fmt.Sprintf("Upgrade to a supported version of %s to avoid extended support costs", resource.Engine)
	}

	// Approaching EOL — any supported target buys the user runway, so
	// the general RecommendedVersion is fine here.
	if rec := usableUpgradeTarget(resource, lifecycle, lifecycle.RecommendedVersion); rec != "" {
		return fmt.Sprintf("Plan upgrade to %s %s within the next 90 days",
			resource.Engine, rec)
	}

	return fmt.Sprintf("Plan upgrade to the latest supported version of %s within the next 90 days", resource.Engine)
}

// versionMatches checks if a resource version matches a lifecycle version.
// endoflife.date uses major.minor cycles (e.g., "8.0") while resources may have
// full versions (e.g., "8.0.35") or prefixed versions (e.g., "k8s-1.33").
func versionMatches(lifecycleVersion, resourceVersion string) bool {
	if lifecycleVersion == resourceVersion {
		return true
	}
	// Strip common prefixes for comparison
	normalized := resourceVersion
	for _, prefix := range []string{"k8s-", "kubernetes-"} {
		normalized = strings.TrimPrefix(normalized, prefix)
	}
	if lifecycleVersion == normalized {
		return true
	}
	return strings.HasPrefix(normalized, lifecycleVersion+".")
}
