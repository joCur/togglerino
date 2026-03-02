package store_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

func setupScheduleTest(t *testing.T) (*store.ScheduleStore, *store.FlagStore, *model.Flag, *model.Environment, string) {
	t.Helper()
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	es := store.NewEnvironmentStore(pool)
	fs := store.NewFlagStore(pool)
	ss := store.NewScheduleStore(pool)
	ctx := context.Background()

	projKey := uniqueKey("sched")
	project, err := ps.Create(ctx, projKey, "Schedule Project", "test")
	if err != nil {
		t.Fatalf("creating project: %v", err)
	}

	env, err := es.Create(ctx, project.ID, "development", "Development")
	if err != nil {
		t.Fatalf("creating env: %v", err)
	}

	flag, err := fs.Create(ctx, project.ID, uniqueKey("schflag"), "Schedule Flag", "test", model.ValueTypeBoolean, model.FlagTypeRelease, json.RawMessage(`false`), []string{}, nil, nil)
	if err != nil {
		t.Fatalf("creating flag: %v", err)
	}

	return ss, fs, flag, env, project.ID
}

func testSnapshot() json.RawMessage {
	return json.RawMessage(`{"enabled":true,"default_variant":"on","variants":[],"targeting_rules":[]}`)
}

func TestScheduleStore_Create(t *testing.T) {
	ss, _, flag, env, _ := setupScheduleTest(t)
	ctx := context.Background()

	scheduledAt := time.Now().Add(1 * time.Hour).Truncate(time.Microsecond)
	sc, err := ss.Create(ctx, flag.ID, env.ID, scheduledAt, testSnapshot(), nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if sc.ID == "" {
		t.Error("expected non-empty ID")
	}
	if sc.FlagID != flag.ID {
		t.Errorf("FlagID: got %q, want %q", sc.FlagID, flag.ID)
	}
	if sc.EnvironmentID != env.ID {
		t.Errorf("EnvironmentID: got %q, want %q", sc.EnvironmentID, env.ID)
	}
	if sc.Status != model.ScheduleStatusPending {
		t.Errorf("Status: got %q, want %q", sc.Status, model.ScheduleStatusPending)
	}
	if !sc.ScheduledAt.Equal(scheduledAt) {
		t.Errorf("ScheduledAt: got %v, want %v", sc.ScheduledAt, scheduledAt)
	}
	if sc.CreatedBy != nil {
		t.Error("expected nil CreatedBy")
	}
}

func TestScheduleStore_Get(t *testing.T) {
	ss, _, flag, env, _ := setupScheduleTest(t)
	ctx := context.Background()

	created, err := ss.Create(ctx, flag.ID, env.ID, time.Now().Add(1*time.Hour), testSnapshot(), nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := ss.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("ID: got %q, want %q", got.ID, created.ID)
	}
}

func TestScheduleStore_Get_NotFound(t *testing.T) {
	pool := testPool(t)
	ss := store.NewScheduleStore(pool)

	_, err := ss.Get(context.Background(), "00000000-0000-0000-0000-000000000000")
	if err == nil {
		t.Fatal("expected error for missing schedule")
	}
}

func TestScheduleStore_ListByFlagEnvironment(t *testing.T) {
	ss, _, flag, env, _ := setupScheduleTest(t)
	ctx := context.Background()

	// Create two schedules
	_, err := ss.Create(ctx, flag.ID, env.ID, time.Now().Add(2*time.Hour), testSnapshot(), nil)
	if err != nil {
		t.Fatalf("Create 1: %v", err)
	}
	_, err = ss.Create(ctx, flag.ID, env.ID, time.Now().Add(1*time.Hour), testSnapshot(), nil)
	if err != nil {
		t.Fatalf("Create 2: %v", err)
	}

	list, err := ss.ListByFlagEnvironment(ctx, flag.ID, env.ID)
	if err != nil {
		t.Fatalf("ListByFlagEnvironment: %v", err)
	}
	if len(list) < 2 {
		t.Fatalf("expected at least 2 schedules, got %d", len(list))
	}
	// Should be ordered by scheduled_at ASC
	if list[0].ScheduledAt.After(list[1].ScheduledAt) {
		t.Error("expected schedules ordered by scheduled_at ASC")
	}
}

func TestScheduleStore_Update(t *testing.T) {
	ss, _, flag, env, _ := setupScheduleTest(t)
	ctx := context.Background()

	created, err := ss.Create(ctx, flag.ID, env.ID, time.Now().Add(1*time.Hour), testSnapshot(), nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	newTime := time.Now().Add(3 * time.Hour).Truncate(time.Microsecond)
	newSnapshot := json.RawMessage(`{"enabled":false,"default_variant":"off","variants":[],"targeting_rules":[]}`)

	updated, err := ss.Update(ctx, created.ID, flag.ID, env.ID, newTime, newSnapshot)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !updated.ScheduledAt.Equal(newTime) {
		t.Errorf("ScheduledAt: got %v, want %v", updated.ScheduledAt, newTime)
	}

	var snap model.ConfigSnapshotPayload
	if err := json.Unmarshal(updated.ConfigSnapshot, &snap); err != nil {
		t.Fatalf("unmarshal snapshot: %v", err)
	}
	if snap.Enabled {
		t.Error("expected updated snapshot to have enabled=false")
	}
}

func TestScheduleStore_Update_WrongOwnership(t *testing.T) {
	ss, _, flag, env, _ := setupScheduleTest(t)
	ctx := context.Background()

	created, err := ss.Create(ctx, flag.ID, env.ID, time.Now().Add(1*time.Hour), testSnapshot(), nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Try updating with wrong flag ID
	_, err = ss.Update(ctx, created.ID, "wrong-flag-id", env.ID, time.Now().Add(2*time.Hour), testSnapshot())
	if err == nil {
		t.Fatal("expected error when updating with wrong flag ID")
	}
}

func TestScheduleStore_Cancel(t *testing.T) {
	ss, _, flag, env, _ := setupScheduleTest(t)
	ctx := context.Background()

	created, err := ss.Create(ctx, flag.ID, env.ID, time.Now().Add(1*time.Hour), testSnapshot(), nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	err = ss.Cancel(ctx, created.ID, flag.ID, env.ID, "test_cancel")
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	got, err := ss.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get after cancel: %v", err)
	}
	if got.Status != model.ScheduleStatusCancelled {
		t.Errorf("Status: got %q, want %q", got.Status, model.ScheduleStatusCancelled)
	}
	if got.CancelledAt == nil {
		t.Error("expected non-nil CancelledAt")
	}
	if got.CancelReason == nil || *got.CancelReason != "test_cancel" {
		t.Errorf("CancelReason: got %v, want 'test_cancel'", got.CancelReason)
	}
}

func TestScheduleStore_Cancel_AlreadyCancelled(t *testing.T) {
	ss, _, flag, env, _ := setupScheduleTest(t)
	ctx := context.Background()

	created, err := ss.Create(ctx, flag.ID, env.ID, time.Now().Add(1*time.Hour), testSnapshot(), nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Cancel once
	err = ss.Cancel(ctx, created.ID, flag.ID, env.ID, "first")
	if err != nil {
		t.Fatalf("first Cancel: %v", err)
	}

	// Cancel again — should return ErrNotFound (not pending anymore)
	err = ss.Cancel(ctx, created.ID, flag.ID, env.ID, "second")
	if err == nil {
		t.Fatal("expected error when cancelling already-cancelled schedule")
	}
}

func TestScheduleStore_CancelByFlag(t *testing.T) {
	ss, _, flag, env, _ := setupScheduleTest(t)
	ctx := context.Background()

	sc1, err := ss.Create(ctx, flag.ID, env.ID, time.Now().Add(1*time.Hour), testSnapshot(), nil)
	if err != nil {
		t.Fatalf("Create 1: %v", err)
	}
	sc2, err := ss.Create(ctx, flag.ID, env.ID, time.Now().Add(2*time.Hour), testSnapshot(), nil)
	if err != nil {
		t.Fatalf("Create 2: %v", err)
	}

	err = ss.CancelByFlag(ctx, flag.ID, "flag_archived")
	if err != nil {
		t.Fatalf("CancelByFlag: %v", err)
	}

	got1, _ := ss.Get(ctx, sc1.ID)
	got2, _ := ss.Get(ctx, sc2.ID)
	if got1.Status != model.ScheduleStatusCancelled {
		t.Errorf("sc1 status: got %q, want cancelled", got1.Status)
	}
	if got2.Status != model.ScheduleStatusCancelled {
		t.Errorf("sc2 status: got %q, want cancelled", got2.Status)
	}
}

func TestScheduleStore_ListDue(t *testing.T) {
	ss, _, flag, env, _ := setupScheduleTest(t)
	ctx := context.Background()

	past := time.Now().Add(-1 * time.Hour)
	future := time.Now().Add(1 * time.Hour)

	// Create one past-due and one future schedule
	_, err := ss.Create(ctx, flag.ID, env.ID, past, testSnapshot(), nil)
	if err != nil {
		t.Fatalf("Create past: %v", err)
	}
	_, err = ss.Create(ctx, flag.ID, env.ID, future, testSnapshot(), nil)
	if err != nil {
		t.Fatalf("Create future: %v", err)
	}

	due, err := ss.ListDue(ctx, time.Now())
	if err != nil {
		t.Fatalf("ListDue: %v", err)
	}

	// Should contain at least the past-due schedule
	found := false
	for _, d := range due {
		if d.FlagID == flag.ID {
			found = true
		}
	}
	if !found {
		t.Error("expected past-due schedule in ListDue results")
	}

	// Future schedule should not be in due list
	for _, d := range due {
		if d.ScheduledAt.After(time.Now()) {
			t.Errorf("found future schedule in due list: scheduled_at=%v", d.ScheduledAt)
		}
	}
}

func TestScheduleStore_Execute(t *testing.T) {
	ss, _, flag, env, _ := setupScheduleTest(t)
	pool := testPool(t)
	ctx := context.Background()

	created, err := ss.Create(ctx, flag.ID, env.ID, time.Now().Add(-1*time.Minute), testSnapshot(), nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	var snap model.ConfigSnapshotPayload
	json.Unmarshal(testSnapshot(), &snap)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	defer tx.Rollback(ctx)

	err = ss.Execute(ctx, tx, created.ID, flag.ID, env.ID, snap)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}

	got, err := ss.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get after execute: %v", err)
	}
	if got.Status != model.ScheduleStatusExecuted {
		t.Errorf("Status: got %q, want %q", got.Status, model.ScheduleStatusExecuted)
	}
	if got.ExecutedAt == nil {
		t.Error("expected non-nil ExecutedAt")
	}
}
