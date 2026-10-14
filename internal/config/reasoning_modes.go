package config

// Reasoning modes inspired by Claude Code's effort levels.
// These provide real reasoning control (low/medium/high/xhigh/max)
// instead of a simple on/off toggle.
//
// The special value "off" (also represented as empty string "") disables
// reasoning entirely.
const (
	ReasoningModeOff    = "off"
	ReasoningModeLow    = "low"
	ReasoningModeMedium = "medium"
	ReasoningModeHigh   = "high"
	ReasoningModeXHigh  = "xhigh"
	ReasoningModeMax    = "max"
)

// AllReasoningModes returns all available reasoning modes in order
// (excluding "off" which is handled separately in most contexts).
func AllReasoningModes() []string {
	return []string{
		ReasoningModeLow,
		ReasoningModeMedium,
		ReasoningModeHigh,
		ReasoningModeXHigh,
		ReasoningModeMax,
	}
}

// FormatReasoningMode returns a human-readable label for a reasoning mode.
func FormatReasoningMode(mode string) string {
	switch mode {
	case "", ReasoningModeOff:
		return "Off"
	case ReasoningModeLow:
		return "Low"
	case ReasoningModeMedium:
		return "Medium"
	case ReasoningModeHigh:
		return "High"
	case ReasoningModeXHigh:
		return "X-High"
	case ReasoningModeMax:
		return "Max"
	default:
		return mode
	}
}

// EffectiveReasoningMode returns the effective reasoning mode for a model
// configuration. It honors the unified ReasoningMode field when set, and
// falls back to the legacy Think/ReasoningEffort fields for backward
// compatibility.
//
// Resolution order:
//  1. Explicit ReasoningMode (takes precedence)
//  2. Think=true → medium (legacy thinking toggle maps to medium)
//  3. ReasoningEffort set → map to appropriate mode
//  4. Empty/"none" effort → off
func (s SelectedModel) EffectiveReasoningMode() string {
	if s.ReasoningMode != "" {
		return NormalizeReasoningMode(s.ReasoningMode)
	}
	// Legacy: Think flag
	if s.Think {
		return ReasoningModeMedium
	}
	// Legacy: ReasoningEffort field
	if s.ReasoningEffort != "" {
		mode := NormalizeReasoningMode(s.ReasoningEffort)
		if mode != "" {
			return mode
		}
	}
	return ReasoningModeOff
}

// NormalizeReasoningMode ensures a mode string is one of the canonical
// values. Unknown values are mapped to the closest canonical equivalent
// or to "off".
func NormalizeReasoningMode(mode string) string {
	switch mode {
	case "", "none", "off", "disabled":
		return ReasoningModeOff
	case "low":
		return ReasoningModeLow
	case "medium":
		return ReasoningModeMedium
	case "high":
		return ReasoningModeHigh
	case "xhigh", "x-high", "extra-high":
		return ReasoningModeXHigh
	case "max", "maximum":
		return ReasoningModeMax
	default:
		// Unrecognized values default to off for safety
		return ReasoningModeOff
	}
}

// IsReasoningEnabled returns true if the model has reasoning enabled
// (any mode other than "off").
func (s SelectedModel) IsReasoningEnabled() bool {
	return s.EffectiveReasoningMode() != ReasoningModeOff
}
