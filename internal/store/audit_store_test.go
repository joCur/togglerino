package store_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

func TestAuditStore_Record(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	as := store.NewAuditStore(pool)
	ctx := context.Background()

	// Create a project for the FK
	key := uniqueKey("audit-rec")
	project, err := ps.Create(ctx, key, "Audit Project", "for audit tests")
	if err != nil {
		t.Fatalf("Create project: %v", err)
	}

	entry := model.AuditEntry{
		ProjectID:  &project.ID,
		Action:     "create",
		EntityType: "project",
		EntityID:   project.Key,
		NewValue:   json.RawMessage(`{"key":"` + project.Key + `","name":"Audit Project"}`),
	}

	err = as.Record(ctx, entry)
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
}

func TestAuditStore_Record_NilOptionalFields(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	as := store.NewAuditStore(pool)
	ctx := context.Background()

	key := uniqueKey("audit-nil")
	project, err := ps.Create(ctx, key, "Nil Fields Project", "testing nil fields")
	if err != nil {
		t.Fatalf("Create project: %v", err)
	}

	entry := model.AuditEntry{
		ProjectID:  &project.ID,
		UserID:     nil,
		Action:     "delete",
		EntityType: "flag",
		EntityID:   "some-flag-key",
		OldValue:   nil,
		NewValue:   nil,
	}

	err = as.Record(ctx, entry)
	if err != nil {
		t.Fatalf("Record with nil fields: %v", err)
	}
}

func TestAuditStore_ListByProject(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	as := store.NewAuditStore(pool)
	ctx := context.Background()

	// Create a project
	key := uniqueKey("audit-list")
	project, err := ps.Create(ctx, key, "List Audit Project", "for list tests")
	if err != nil {
		t.Fatalf("Create project: %v", err)
	}

	// Record multiple entries
	for i := 0; i < 3; i++ {
		entry := model.AuditEntry{
			ProjectID:  &project.ID,
			Action:     "update",
			EntityType: "flag",
			EntityID:   uniqueKey("flag"),
			NewValue:   json.RawMessage(`{"iteration":` + string(rune('0'+i)) + `}`),
		}
		if err := as.Record(ctx, entry); err != nil {
			t.Fatalf("Record %d: %v", i, err)
		}
	}

	// List with limit and offset
	entries, err := as.ListByProject(ctx, project.ID, 50, 0)
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}

	if len(entries) < 3 {
		t.Fatalf("expected at least 3 entries, got %d", len(entries))
	}

	// Verify ordering is created_at DESC
	for i := 1; i < len(entries); i++ {
		if entries[i].CreatedAt.After(entries[i-1].CreatedAt) {
			t.Error("entries should be ordered by created_at DESC")
			break
		}
	}

	// Verify fields are populated
	for _, e := range entries {
		if e.ID == "" {
			t.Error("expected non-empty ID")
		}
		if e.ProjectID == nil || *e.ProjectID != project.ID {
			t.Errorf("expected project_id %q", project.ID)
		}
		if e.Action == "" {
			t.Error("expected non-empty Action")
		}
		if e.EntityType == "" {
			t.Error("expected non-empty EntityType")
		}
		if e.EntityID == "" {
			t.Error("expected non-empty EntityID")
		}
		if e.CreatedAt.IsZero() {
			t.Error("expected non-zero CreatedAt")
		}
	}
}

func TestAuditStore_ListByProject_Pagination(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	as := store.NewAuditStore(pool)
	ctx := context.Background()

	// Create a project
	key := uniqueKey("audit-page")
	project, err := ps.Create(ctx, key, "Pagination Audit Project", "for pagination tests")
	if err != nil {
		t.Fatalf("Create project: %v", err)
	}

	// Record 5 entries
	for i := 0; i < 5; i++ {
		entry := model.AuditEntry{
			ProjectID:  &project.ID,
			Action:     "create",
			EntityType: "flag",
			EntityID:   uniqueKey("pflag"),
		}
		if err := as.Record(ctx, entry); err != nil {
			t.Fatalf("Record %d: %v", i, err)
		}
	}

	// Fetch first page (limit=2)
	page1, err := as.ListByProject(ctx, project.ID, 2, 0)
	if err != nil {
		t.Fatalf("ListByProject page1: %v", err)
	}
	if len(page1) != 2 {
		t.Fatalf("page1: expected 2 entries, got %d", len(page1))
	}

	// Fetch second page (limit=2, offset=2)
	page2, err := as.ListByProject(ctx, project.ID, 2, 2)
	if err != nil {
		t.Fatalf("ListByProject page2: %v", err)
	}
	if len(page2) != 2 {
		t.Fatalf("page2: expected 2 entries, got %d", len(page2))
	}

	// Entries on page1 and page2 should not overlap
	if page1[0].ID == page2[0].ID {
		t.Error("page1 and page2 should have different entries")
	}
}

func TestAuditStore_ListByProject_Empty(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	as := store.NewAuditStore(pool)
	ctx := context.Background()

	// Create a project with no audit entries
	key := uniqueKey("audit-empty")
	project, err := ps.Create(ctx, key, "Empty Audit Project", "no entries")
	if err != nil {
		t.Fatalf("Create project: %v", err)
	}

	entries, err := as.ListByProject(ctx, project.ID, 50, 0)
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}

	if entries != nil {
		t.Errorf("expected nil for empty result, got %d entries", len(entries))
	}
}

func TestAuditStore_Record_WithNewFields(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	es := store.NewEnvironmentStore(pool)
	as := store.NewAuditStore(pool)
	ctx := context.Background()

	key := uniqueKey("audit-new")
	project, err := ps.Create(ctx, key, "New Fields Project", "testing new fields")
	if err != nil {
		t.Fatalf("Create project: %v", err)
	}

	env, err := es.Create(ctx, project.ID, "dev", "Development")
	if err != nil {
		t.Fatalf("Create environment: %v", err)
	}
	envID := env.ID
	email := "test@example.com"

	entry := model.AuditEntry{
		ProjectID:     &project.ID,
		UserEmail:     &email,
		EnvironmentID: &envID,
		Action:        "update",
		EntityType:    "flag_config",
		EntityID:      "test-flag",
		NewValue:      json.RawMessage(`{"enabled":true}`),
	}

	err = as.Record(ctx, entry)
	if err != nil {
		t.Fatalf("Record with new fields: %v", err)
	}
}

func TestAuditStore_GetByID(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	as := store.NewAuditStore(pool)
	ctx := context.Background()

	key := uniqueKey("audit-get")
	project, err := ps.Create(ctx, key, "GetByID Project", "for GetByID test")
	if err != nil {
		t.Fatalf("Create project: %v", err)
	}

	entry := model.AuditEntry{
		ProjectID:  &project.ID,
		Action:     "create",
		EntityType: "flag",
		EntityID:   "my-flag",
		NewValue:   json.RawMessage(`{"key":"my-flag"}`),
	}
	if err := as.Record(ctx, entry); err != nil {
		t.Fatalf("Record: %v", err)
	}

	// List to find the ID
	entries, err := as.ListByProject(ctx, project.ID, 1, 0)
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least 1 entry")
	}

	got, err := as.GetByID(ctx, entries[0].ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.ID != entries[0].ID {
		t.Errorf("expected ID %q, got %q", entries[0].ID, got.ID)
	}
	if got.EntityID != "my-flag" {
		t.Errorf("expected EntityID 'my-flag', got %q", got.EntityID)
	}
}

func TestAuditStore_ListByFlag(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	as := store.NewAuditStore(pool)
	ctx := context.Background()

	key := uniqueKey("audit-flag")
	project, err := ps.Create(ctx, key, "ListByFlag Project", "for ListByFlag test")
	if err != nil {
		t.Fatalf("Create project: %v", err)
	}

	// Record entries for two different flags
	for _, fk := range []string{"flag-a", "flag-b"} {
		for i := 0; i < 3; i++ {
			entry := model.AuditEntry{
				ProjectID:  &project.ID,
				Action:     "update",
				EntityType: "flag_config",
				EntityID:   fk,
				NewValue:   json.RawMessage(`{"enabled":true}`),
			}
			if err := as.Record(ctx, entry); err != nil {
				t.Fatalf("Record %s-%d: %v", fk, i, err)
			}
		}
	}

	// Also record a flag-level entry for flag-a
	if err := as.Record(ctx, model.AuditEntry{
		ProjectID:  &project.ID,
		Action:     "create",
		EntityType: "flag",
		EntityID:   "flag-a",
		NewValue:   json.RawMessage(`{"key":"flag-a"}`),
	}); err != nil {
		t.Fatalf("Record flag create: %v", err)
	}

	// ListByFlag for flag-a should return 4 entries (3 config + 1 flag)
	entries, err := as.ListByFlag(ctx, project.ID, "flag-a", nil, 50, 0)
	if err != nil {
		t.Fatalf("ListByFlag: %v", err)
	}
	if len(entries) != 4 {
		t.Fatalf("expected 4 entries for flag-a, got %d", len(entries))
	}

	// All should be for flag-a
	for _, e := range entries {
		if e.EntityID != "flag-a" {
			t.Errorf("expected EntityID 'flag-a', got %q", e.EntityID)
		}
	}

	// Verify ordering is created_at DESC
	for i := 1; i < len(entries); i++ {
		if entries[i].CreatedAt.After(entries[i-1].CreatedAt) {
			t.Error("entries should be ordered by created_at DESC")
			break
		}
	}
}

func TestAuditStore_ListByFlag_EnvFilter(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	es := store.NewEnvironmentStore(pool)
	as := store.NewAuditStore(pool)
	ctx := context.Background()

	key := uniqueKey("audit-env")
	project, err := ps.Create(ctx, key, "EnvFilter Project", "for env filter test")
	if err != nil {
		t.Fatalf("Create project: %v", err)
	}

	env1, err := es.Create(ctx, project.ID, "dev", "Development")
	if err != nil {
		t.Fatalf("Create environment 1: %v", err)
	}
	env2, err := es.Create(ctx, project.ID, "staging", "Staging")
	if err != nil {
		t.Fatalf("Create environment 2: %v", err)
	}
	env1ID := env1.ID
	env2ID := env2.ID

	// Record 2 entries for env1, 1 for env2
	for i := 0; i < 2; i++ {
		if err := as.Record(ctx, model.AuditEntry{
			ProjectID:     &project.ID,
			EnvironmentID: &env1ID,
			Action:        "update",
			EntityType:    "flag_config",
			EntityID:      "my-flag",
			NewValue:      json.RawMessage(`{"enabled":true}`),
		}); err != nil {
			t.Fatalf("Record env1-%d: %v", i, err)
		}
	}
	if err := as.Record(ctx, model.AuditEntry{
		ProjectID:     &project.ID,
		EnvironmentID: &env2ID,
		Action:        "update",
		EntityType:    "flag_config",
		EntityID:      "my-flag",
		NewValue:      json.RawMessage(`{"enabled":false}`),
	}); err != nil {
		t.Fatalf("Record env2: %v", err)
	}

	// Filter by env1 should return 2
	entries, err := as.ListByFlag(ctx, project.ID, "my-flag", &env1ID, 50, 0)
	if err != nil {
		t.Fatalf("ListByFlag with env filter: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries for env1, got %d", len(entries))
	}
}

func TestAuditStore_Record_WithBatchID(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	as := store.NewAuditStore(pool)
	ctx := context.Background()

	key := uniqueKey("audit-batch")
	project, err := ps.Create(ctx, key, "Batch ID Project", "testing batch_id")
	if err != nil {
		t.Fatalf("Create project: %v", err)
	}

	batchID := "550e8400-e29b-41d4-a716-446655440000"
	entry := model.AuditEntry{
		ProjectID:  &project.ID,
		BatchID:    &batchID,
		Action:     "enable",
		EntityType: "flag_config",
		EntityID:   "test-flag",
		NewValue:   json.RawMessage(`{"enabled":true}`),
	}

	err = as.Record(ctx, entry)
	if err != nil {
		t.Fatalf("Record with batch_id: %v", err)
	}

	// Verify the batch_id was stored and can be read back
	entries, err := as.ListByProject(ctx, project.ID, 1, 0)
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least 1 entry")
	}
	if entries[0].BatchID == nil || *entries[0].BatchID != batchID {
		t.Errorf("BatchID: got %v, want %q", entries[0].BatchID, batchID)
	}
}
