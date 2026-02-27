package staleness

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/togglerino/togglerino/internal/model"
)

// FlagStore is the interface for flag operations needed by the staleness checker.
type FlagStore interface {
	ListNonArchived(ctx context.Context) ([]model.Flag, error)
	SetLifecycleStatus(ctx context.Context, flagID string, status model.LifecycleStatus) (*model.Flag, error)
}

// SettingsStore is the interface for project settings operations needed by the staleness checker.
type SettingsStore interface {
	GetAll(ctx context.Context) (map[string]*model.ProjectSettings, error)
}

// AuditRecorder is the interface for recording audit events.
type AuditRecorder interface {
	Record(ctx context.Context, entry model.AuditEntry) error
}

// CacheRefresher is the interface for refreshing the in-memory flag cache.
type CacheRefresher interface {
	LoadAll(ctx context.Context) error
}

// Checker periodically checks flags and promotes them through lifecycle stages.
type Checker struct {
	flags    FlagStore
	settings SettingsStore
	audit    AuditRecorder
	cache    CacheRefresher
	interval time.Duration
	now      func() time.Time // injectable for testing
}

// NewChecker creates a new staleness checker.
func NewChecker(flags FlagStore, settings SettingsStore, audit AuditRecorder, cache CacheRefresher, interval time.Duration) *Checker {
	return &Checker{flags: flags, settings: settings, audit: audit, cache: cache, interval: interval, now: time.Now}
}

// Run starts the staleness checker loop. Blocks until ctx is cancelled.
func (c *Checker) Run(ctx context.Context) {
	slog.Info("staleness checker started", "interval", c.interval)

	// Run immediately on startup
	c.tick(ctx)

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("staleness checker stopped")
			return
		case <-ticker.C:
			c.tick(ctx)
		}
	}
}

const gracePeriod = 14 * 24 * time.Hour // 14 days

func (c *Checker) tick(ctx context.Context) {
	flags, err := c.flags.ListNonArchived(ctx)
	if err != nil {
		slog.Error("staleness checker: failed to list flags", "error", err)
		return
	}

	allSettings, err := c.settings.GetAll(ctx)
	if err != nil {
		slog.Error("staleness checker: failed to load settings", "error", err)
		return
	}

	promoted := 0
	now := c.now()
	for _, f := range flags {
		settings := allSettings[f.ProjectID]
		ps := &model.ProjectSettings{FlagLifetimes: nil}
		if settings != nil {
			ps = settings
		}

		lifetime := ps.GetLifetime(f.FlagType)
		if lifetime == nil {
			// Permanent flag type — skip
			continue
		}

		expectedEnd := f.CreatedAt.Add(time.Duration(*lifetime) * 24 * time.Hour)

		switch f.LifecycleStatus {
		case model.LifecycleActive:
			if now.After(expectedEnd) {
				c.promote(ctx, f, model.LifecyclePotentiallyStale)
				promoted++
			}
		case model.LifecyclePotentiallyStale:
			if f.LifecycleStatusChangedAt != nil && now.After(f.LifecycleStatusChangedAt.Add(gracePeriod)) {
				c.promote(ctx, f, model.LifecycleStale)
				promoted++
			}
		case model.LifecycleStale:
			// Already stale — nothing to do
		}
	}

	// Refresh in-memory cache if any flags were promoted
	if promoted > 0 {
		if err := c.cache.LoadAll(ctx); err != nil {
			slog.Error("staleness checker: failed to refresh cache", "error", err)
		}
	}
}

func (c *Checker) promote(ctx context.Context, flag model.Flag, newStatus model.LifecycleStatus) {
	updated, err := c.flags.SetLifecycleStatus(ctx, flag.ID, newStatus)
	if err != nil {
		slog.Error("staleness checker: failed to update status",
			"flag", flag.Key, "to", string(newStatus), "error", err)
		return
	}

	oldVal, _ := json.Marshal(map[string]string{"lifecycle_status": string(flag.LifecycleStatus)})
	newVal, _ := json.Marshal(map[string]string{"lifecycle_status": string(updated.LifecycleStatus)})

	if err := c.audit.Record(ctx, model.AuditEntry{
		ProjectID:  &flag.ProjectID,
		Action:     "staleness_change",
		EntityType: "flag",
		EntityID:   flag.Key,
		OldValue:   oldVal,
		NewValue:   newVal,
	}); err != nil {
		slog.Warn("staleness checker: failed to record audit", "error", err)
	}

	slog.Info("staleness checker: promoted flag",
		"flag", flag.Key, "from", string(flag.LifecycleStatus), "to", string(newStatus))
}
