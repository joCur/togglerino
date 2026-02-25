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
