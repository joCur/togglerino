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

// ProjectSettings holds per-project configuration.
type ProjectSettings struct {
	ID            string            `json:"id"`
	ProjectID     string            `json:"project_id"`
	FlagLifetimes map[FlagType]*int `json:"flag_lifetimes"`
	UpdatedAt     time.Time         `json:"updated_at"`
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
