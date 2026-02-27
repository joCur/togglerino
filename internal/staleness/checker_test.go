package staleness

import (
	"context"
	"testing"
	"time"

	"github.com/togglerino/togglerino/internal/model"
)

// --- Mock stores ---

type mockFlagStore struct {
	flags     []model.Flag
	promoted  []promotion
	returnErr error
}

type promotion struct {
	flagID string
	status model.LifecycleStatus
}

func (m *mockFlagStore) ListNonArchived(_ context.Context) ([]model.Flag, error) {
	return m.flags, m.returnErr
}

func (m *mockFlagStore) SetLifecycleStatus(_ context.Context, flagID string, status model.LifecycleStatus) (*model.Flag, error) {
	m.promoted = append(m.promoted, promotion{flagID, status})
	return &model.Flag{ID: flagID, LifecycleStatus: status}, nil
}

type mockSettingsStore struct {
	settings map[string]*model.ProjectSettings
}

func (m *mockSettingsStore) GetAll(_ context.Context) (map[string]*model.ProjectSettings, error) {
	if m.settings == nil {
		return map[string]*model.ProjectSettings{}, nil
	}
	return m.settings, nil
}

type mockAudit struct {
	entries []model.AuditEntry
}

func (m *mockAudit) Record(_ context.Context, entry model.AuditEntry) error {
	m.entries = append(m.entries, entry)
	return nil
}

type mockCache struct {
	refreshCount int
}

func (m *mockCache) LoadAll(_ context.Context) error {
	m.refreshCount++
	return nil
}

// --- Helpers ---

func intPtr(v int) *int { return &v }

func timePtr(t time.Time) *time.Time { return &t }

func makeFlag(key, projectID string, flagType model.FlagType, status model.LifecycleStatus, createdAt time.Time, statusChangedAt *time.Time) model.Flag {
	return model.Flag{
		ID:                       key + "-id",
		ProjectID:                projectID,
		Key:                      key,
		FlagType:                 flagType,
		LifecycleStatus:          status,
		CreatedAt:                createdAt,
		LifecycleStatusChangedAt: statusChangedAt,
	}
}

// --- Tests ---

func TestTick_ActiveWithinLifetime_NoPromotion(t *testing.T) {
	now := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	flags := &mockFlagStore{
		flags: []model.Flag{
			makeFlag("new-flag", "proj-1", model.FlagTypeRelease, model.LifecycleActive, now.Add(-10*24*time.Hour), nil),
		},
	}
	cache := &mockCache{}
	c := &Checker{
		flags:    flags,
		settings: &mockSettingsStore{},
		audit:    &mockAudit{},
		cache:    cache,
		now:      func() time.Time { return now },
	}

	c.tick(context.Background())

	if len(flags.promoted) != 0 {
		t.Errorf("expected no promotions, got %d", len(flags.promoted))
	}
	if cache.refreshCount != 0 {
		t.Errorf("expected no cache refresh, got %d", cache.refreshCount)
	}
}

func TestTick_ActivePastLifetime_PromoteToPotentiallyStale(t *testing.T) {
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	// Created 50 days ago, default release lifetime is 40 days
	flags := &mockFlagStore{
		flags: []model.Flag{
			makeFlag("old-flag", "proj-1", model.FlagTypeRelease, model.LifecycleActive, now.Add(-50*24*time.Hour), nil),
		},
	}
	cache := &mockCache{}
	c := &Checker{
		flags:    flags,
		settings: &mockSettingsStore{},
		audit:    &mockAudit{},
		cache:    cache,
		now:      func() time.Time { return now },
	}

	c.tick(context.Background())

	if len(flags.promoted) != 1 {
		t.Fatalf("expected 1 promotion, got %d", len(flags.promoted))
	}
	if flags.promoted[0].status != model.LifecyclePotentiallyStale {
		t.Errorf("expected promotion to potentially_stale, got %s", flags.promoted[0].status)
	}
	if cache.refreshCount != 1 {
		t.Errorf("expected 1 cache refresh, got %d", cache.refreshCount)
	}
}

func TestTick_PotentiallyStaleWithinGracePeriod_NoPromotion(t *testing.T) {
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	changedAt := now.Add(-5 * 24 * time.Hour) // 5 days ago (within 14-day grace)
	flags := &mockFlagStore{
		flags: []model.Flag{
			makeFlag("stale-ish", "proj-1", model.FlagTypeRelease, model.LifecyclePotentiallyStale, now.Add(-60*24*time.Hour), timePtr(changedAt)),
		},
	}
	c := &Checker{
		flags:    flags,
		settings: &mockSettingsStore{},
		audit:    &mockAudit{},
		cache:    &mockCache{},
		now:      func() time.Time { return now },
	}

	c.tick(context.Background())

	if len(flags.promoted) != 0 {
		t.Errorf("expected no promotions during grace period, got %d", len(flags.promoted))
	}
}

func TestTick_PotentiallyStalePastGracePeriod_PromoteToStale(t *testing.T) {
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	changedAt := now.Add(-15 * 24 * time.Hour) // 15 days ago (past 14-day grace)
	flags := &mockFlagStore{
		flags: []model.Flag{
			makeFlag("really-stale", "proj-1", model.FlagTypeRelease, model.LifecyclePotentiallyStale, now.Add(-60*24*time.Hour), timePtr(changedAt)),
		},
	}
	audit := &mockAudit{}
	c := &Checker{
		flags:    flags,
		settings: &mockSettingsStore{},
		audit:    audit,
		cache:    &mockCache{},
		now:      func() time.Time { return now },
	}

	c.tick(context.Background())

	if len(flags.promoted) != 1 {
		t.Fatalf("expected 1 promotion, got %d", len(flags.promoted))
	}
	if flags.promoted[0].status != model.LifecycleStale {
		t.Errorf("expected promotion to stale, got %s", flags.promoted[0].status)
	}
	if len(audit.entries) != 1 {
		t.Errorf("expected 1 audit entry, got %d", len(audit.entries))
	}
}

func TestTick_PermanentFlagType_Skipped(t *testing.T) {
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	flags := &mockFlagStore{
		flags: []model.Flag{
			// kill-switch has nil lifetime (permanent) — should never be promoted
			makeFlag("safety", "proj-1", model.FlagTypeKillSwitch, model.LifecycleActive, now.Add(-365*24*time.Hour), nil),
			makeFlag("perms", "proj-1", model.FlagTypePermission, model.LifecycleActive, now.Add(-365*24*time.Hour), nil),
		},
	}
	c := &Checker{
		flags:    flags,
		settings: &mockSettingsStore{},
		audit:    &mockAudit{},
		cache:    &mockCache{},
		now:      func() time.Time { return now },
	}

	c.tick(context.Background())

	if len(flags.promoted) != 0 {
		t.Errorf("expected no promotions for permanent flag types, got %d", len(flags.promoted))
	}
}

func TestTick_ProjectSettingsOverrideDefaults(t *testing.T) {
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	// Flag is 15 days old, default release lifetime is 40 days (would not promote)
	// But project overrides release to 10 days (should promote)
	flags := &mockFlagStore{
		flags: []model.Flag{
			makeFlag("short-lived", "proj-custom", model.FlagTypeRelease, model.LifecycleActive, now.Add(-15*24*time.Hour), nil),
		},
	}
	settings := &mockSettingsStore{
		settings: map[string]*model.ProjectSettings{
			"proj-custom": {
				ProjectID:     "proj-custom",
				FlagLifetimes: map[model.FlagType]*int{model.FlagTypeRelease: intPtr(10)},
			},
		},
	}
	c := &Checker{
		flags:    flags,
		settings: settings,
		audit:    &mockAudit{},
		cache:    &mockCache{},
		now:      func() time.Time { return now },
	}

	c.tick(context.Background())

	if len(flags.promoted) != 1 {
		t.Fatalf("expected 1 promotion with custom lifetime, got %d", len(flags.promoted))
	}
	if flags.promoted[0].status != model.LifecyclePotentiallyStale {
		t.Errorf("expected promotion to potentially_stale, got %s", flags.promoted[0].status)
	}
}

func TestTick_StaleFlag_NoAction(t *testing.T) {
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	flags := &mockFlagStore{
		flags: []model.Flag{
			makeFlag("already-stale", "proj-1", model.FlagTypeRelease, model.LifecycleStale, now.Add(-90*24*time.Hour), timePtr(now.Add(-30*24*time.Hour))),
		},
	}
	c := &Checker{
		flags:    flags,
		settings: &mockSettingsStore{},
		audit:    &mockAudit{},
		cache:    &mockCache{},
		now:      func() time.Time { return now },
	}

	c.tick(context.Background())

	if len(flags.promoted) != 0 {
		t.Errorf("expected no promotions for already-stale flag, got %d", len(flags.promoted))
	}
}

func TestTick_OperationalFlagShorterLifetime(t *testing.T) {
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	// Operational default is 7 days, flag is 10 days old — should promote
	flags := &mockFlagStore{
		flags: []model.Flag{
			makeFlag("migration", "proj-1", model.FlagTypeOperational, model.LifecycleActive, now.Add(-10*24*time.Hour), nil),
		},
	}
	c := &Checker{
		flags:    flags,
		settings: &mockSettingsStore{},
		audit:    &mockAudit{},
		cache:    &mockCache{},
		now:      func() time.Time { return now },
	}

	c.tick(context.Background())

	if len(flags.promoted) != 1 {
		t.Fatalf("expected 1 promotion for operational flag, got %d", len(flags.promoted))
	}
	if flags.promoted[0].status != model.LifecyclePotentiallyStale {
		t.Errorf("expected potentially_stale, got %s", flags.promoted[0].status)
	}
}
