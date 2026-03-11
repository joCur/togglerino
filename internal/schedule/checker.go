package schedule

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/togglerino/togglerino/internal/model"
)

// ScheduleStore is the narrow interface for schedule operations needed by the checker.
type ScheduleStore interface {
	ListDue(ctx context.Context, now time.Time) ([]model.ScheduledFlagChange, error)
	Execute(ctx context.Context, tx pgx.Tx, scheduleID, flagID, environmentID string, snapshot model.ConfigSnapshotPayload) error
}

// FlagEnvLookup resolves project/environment/flag keys from IDs.
type FlagEnvLookup interface {
	ProjectKeyByFlagID(ctx context.Context, flagID string) (string, error)
	ProjectIDByFlagID(ctx context.Context, flagID string) (string, error)
	FlagKeyByID(ctx context.Context, flagID string) (string, error)
	EnvKeyByID(ctx context.Context, environmentID string) (string, error)
}

// TxBeginner starts a database transaction.
type TxBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// CacheRefresher refreshes the in-memory evaluation cache for one project/env.
type CacheRefresher interface {
	Refresh(ctx context.Context, projectKey, envKey string) error
}

// EventBroadcaster broadcasts flag update events to connected SDK clients.
type EventBroadcaster interface {
	Broadcast(projectKey, envKey string, flagKey string, enabled bool, defaultVariant string)
}

// AuditRecorder records audit log entries.
type AuditRecorder interface {
	Record(ctx context.Context, entry model.AuditEntry) error
}

// LockChecker checks if a flag is locked in an environment.
type LockChecker interface {
	IsEnvironmentConfigLocked(ctx context.Context, flagID, environmentID string) (bool, error)
}

// Checker periodically executes due scheduled flag changes.
type Checker struct {
	schedules   ScheduleStore
	lookup      FlagEnvLookup
	db          TxBeginner
	cache       CacheRefresher
	broadcaster EventBroadcaster
	audit       AuditRecorder
	locks       LockChecker
	interval    time.Duration
	now         func() time.Time // injectable for testing
}

// NewChecker creates a new schedule checker.
func NewChecker(
	schedules ScheduleStore,
	lookup FlagEnvLookup,
	db TxBeginner,
	cache CacheRefresher,
	broadcaster EventBroadcaster,
	audit AuditRecorder,
	locks LockChecker,
	interval time.Duration,
) *Checker {
	return &Checker{
		schedules:   schedules,
		lookup:      lookup,
		db:          db,
		cache:       cache,
		broadcaster: broadcaster,
		audit:       audit,
		locks:       locks,
		interval:    interval,
		now:         time.Now,
	}
}

// Run starts the checker loop. Blocks until ctx is cancelled.
func (c *Checker) Run(ctx context.Context) {
	slog.Info("schedule checker started", "interval", c.interval)

	c.tick(ctx)

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("schedule checker stopped")
			return
		case <-ticker.C:
			c.tick(ctx)
		}
	}
}

func (c *Checker) tick(ctx context.Context) {
	due, err := c.schedules.ListDue(ctx, c.now())
	if err != nil {
		slog.Error("schedule checker: failed to list due schedules", "error", err)
		return
	}

	for _, sc := range due {
		c.execute(ctx, sc)
	}
}

func (c *Checker) execute(ctx context.Context, sc model.ScheduledFlagChange) {
	// Check if flag is locked in target environment
	if c.locks != nil {
		locked, err := c.locks.IsEnvironmentConfigLocked(ctx, sc.FlagID, sc.EnvironmentID)
		if err != nil {
			slog.Warn("schedule checker: failed to check lock status, skipping",
				"schedule_id", sc.ID, "error", err)
			return
		}
		if locked {
			slog.Warn("schedule checker: flag is locked, skipping scheduled change",
				"schedule_id", sc.ID, "flag_id", sc.FlagID, "env_id", sc.EnvironmentID)
			return
		}
	}

	var snapshot model.ConfigSnapshotPayload
	if err := json.Unmarshal(sc.ConfigSnapshot, &snapshot); err != nil {
		slog.Error("schedule checker: failed to unmarshal config snapshot",
			"schedule_id", sc.ID, "error", err)
		return
	}

	if snapshot.Variants == nil {
		snapshot.Variants = json.RawMessage(`[]`)
	}
	if snapshot.TargetingRules == nil {
		snapshot.TargetingRules = json.RawMessage(`[]`)
	}

	tx, err := c.db.Begin(ctx)
	if err != nil {
		slog.Error("schedule checker: failed to begin transaction",
			"schedule_id", sc.ID, "error", err)
		return
	}
	defer tx.Rollback(ctx)

	if err := c.schedules.Execute(ctx, tx, sc.ID, sc.FlagID, sc.EnvironmentID, snapshot); err != nil {
		slog.Error("schedule checker: failed to execute schedule",
			"schedule_id", sc.ID, "error", err)
		return
	}

	if err := tx.Commit(ctx); err != nil {
		slog.Error("schedule checker: failed to commit execution",
			"schedule_id", sc.ID, "error", err)
		return
	}

	slog.Info("schedule checker: executed scheduled change",
		"schedule_id", sc.ID, "flag_id", sc.FlagID, "env_id", sc.EnvironmentID)

	c.postExecute(ctx, sc, snapshot)
}

func (c *Checker) postExecute(ctx context.Context, sc model.ScheduledFlagChange, snapshot model.ConfigSnapshotPayload) {
	// Resolve all keys independently — don't let one failure prevent other steps
	projectKey, pkErr := c.lookup.ProjectKeyByFlagID(ctx, sc.FlagID)
	if pkErr != nil {
		slog.Warn("schedule checker: failed to resolve project key",
			"flag_id", sc.FlagID, "error", pkErr)
	}
	envKey, ekErr := c.lookup.EnvKeyByID(ctx, sc.EnvironmentID)
	if ekErr != nil {
		slog.Warn("schedule checker: failed to resolve env key",
			"env_id", sc.EnvironmentID, "error", ekErr)
	}
	flagKey, fkErr := c.lookup.FlagKeyByID(ctx, sc.FlagID)
	if fkErr != nil {
		slog.Warn("schedule checker: failed to resolve flag key",
			"flag_id", sc.FlagID, "error", fkErr)
	}

	// Cache refresh + SSE broadcast (requires project key + env key)
	if pkErr == nil && ekErr == nil {
		if err := c.cache.Refresh(ctx, projectKey, envKey); err != nil {
			slog.Warn("schedule checker: failed to refresh cache",
				"project", projectKey, "env", envKey, "error", err)
		}
		if fkErr == nil {
			c.broadcaster.Broadcast(projectKey, envKey, flagKey, snapshot.Enabled, snapshot.DefaultVariant)
		}
	}

	// Best-effort audit logging
	projectID, pidErr := c.lookup.ProjectIDByFlagID(ctx, sc.FlagID)
	if pidErr != nil {
		slog.Warn("schedule checker: failed to resolve project ID for audit",
			"flag_id", sc.FlagID, "error", pidErr)
		return
	}
	entityID := sc.FlagID
	if fkErr == nil {
		entityID = flagKey
	}
	newVal, _ := json.Marshal(snapshot)
	if err := c.audit.Record(ctx, model.AuditEntry{
		ProjectID:  &projectID,
		Action:     "schedule_executed",
		EntityType: "flag_config",
		EntityID:   entityID,
		NewValue:   newVal,
	}); err != nil {
		slog.Warn("schedule checker: failed to record audit", "error", err)
	}
}
