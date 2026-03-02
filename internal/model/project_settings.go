package model

import "time"

// DefaultFlagLifetimes returns the default expected lifetimes (in days) per flag type.
// nil means permanent (never stale).
func DefaultFlagLifetimes() map[FlagType]*int {
	return map[FlagType]*int{
		FlagTypeRelease:     intPtr(40),
		FlagTypeExperiment:  intPtr(40),
		FlagTypeOperational: intPtr(7),
		FlagTypeKillSwitch:  nil,
		FlagTypePermission:  nil,
	}
}

func intPtr(n int) *int { return &n }

// EnvironmentDefault holds the default flag configuration for an environment.
type EnvironmentDefault struct {
	Enabled bool `json:"enabled"`
}

// DefaultEnvironmentDefaults returns the hardcoded fallback defaults.
// "development" is enabled; all others are disabled.
func DefaultEnvironmentDefaults() map[string]EnvironmentDefault {
	return map[string]EnvironmentDefault{
		"development": {Enabled: true},
		"staging":     {Enabled: false},
		"production":  {Enabled: false},
	}
}

// ProjectSettings holds per-project configuration.
type ProjectSettings struct {
	ID                  string                         `json:"id"`
	ProjectID           string                         `json:"project_id"`
	FlagLifetimes       map[FlagType]*int              `json:"flag_lifetimes"`
	EnvironmentDefaults map[string]EnvironmentDefault  `json:"environment_defaults,omitempty"`
	UpdatedAt           time.Time                      `json:"updated_at"`
}

// GetLifetime returns the expected lifetime in days for a flag type,
// using the project setting if available, otherwise the global default.
func (ps *ProjectSettings) GetLifetime(ft FlagType) *int {
	if ps != nil && ps.FlagLifetimes != nil {
		if v, ok := ps.FlagLifetimes[ft]; ok {
			return v
		}
	}
	return DefaultFlagLifetimes()[ft]
}

// ResolveEnvironmentDefaults merges hardcoded fallbacks with project-level
// settings and optional per-request overrides. Returns a map of env key → enabled.
func (ps *ProjectSettings) ResolveEnvironmentDefaults(envKeys []string, overrides map[string]EnvironmentDefault) map[string]bool {
	result := make(map[string]bool, len(envKeys))
	hardcoded := DefaultEnvironmentDefaults()

	for _, key := range envKeys {
		// Layer 1: hardcoded fallback (development=true, everything else=false)
		if hc, ok := hardcoded[key]; ok {
			result[key] = hc.Enabled
		} else {
			result[key] = false
		}

		// Layer 2: project-level setting
		if ps != nil && ps.EnvironmentDefaults != nil {
			if pd, ok := ps.EnvironmentDefaults[key]; ok {
				result[key] = pd.Enabled
			}
		}

		// Layer 3: per-request override
		if overrides != nil {
			if ov, ok := overrides[key]; ok {
				result[key] = ov.Enabled
			}
		}
	}
	return result
}
