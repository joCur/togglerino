package store_test

import (
	"context"
	"testing"

	"github.com/togglerino/togglerino/internal/store"
)

func TestWebhookStore_Create(t *testing.T) {
	pool := testPool(t)
	ws := store.NewWebhookStore(pool)
	ps := store.NewProjectStore(pool)

	ctx := context.Background()
	project, err := ps.Create(ctx, uniqueKey("wh-test"), "Webhook Test", "")
	if err != nil {
		t.Fatalf("creating project: %v", err)
	}

	wh, err := ws.Create(ctx, project.ID, "My Hook", "https://example.com/hook", "whsec_test123", []string{"flag.created", "flag.updated"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if wh.Name != "My Hook" {
		t.Errorf("Name = %q, want %q", wh.Name, "My Hook")
	}
	if wh.URL != "https://example.com/hook" {
		t.Errorf("URL = %q, want %q", wh.URL, "https://example.com/hook")
	}
	if !wh.Enabled {
		t.Error("Enabled = false, want true")
	}
	if len(wh.EventTypes) != 2 {
		t.Errorf("EventTypes len = %d, want 2", len(wh.EventTypes))
	}
}

func TestWebhookStore_ListByProject(t *testing.T) {
	pool := testPool(t)
	ws := store.NewWebhookStore(pool)
	ps := store.NewProjectStore(pool)

	ctx := context.Background()
	project, _ := ps.Create(ctx, uniqueKey("wh-list"), "WH List", "")
	ws.Create(ctx, project.ID, "Hook 1", "https://a.com/hook", "whsec_a", []string{"flag.created"})
	ws.Create(ctx, project.ID, "Hook 2", "https://b.com/hook", "whsec_b", []string{"flag.updated"})

	hooks, total, err := ws.ListByProject(ctx, project.ID, 50, 0)
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(hooks) != 2 {
		t.Errorf("len = %d, want 2", len(hooks))
	}
}

func TestWebhookStore_GetByID(t *testing.T) {
	pool := testPool(t)
	ws := store.NewWebhookStore(pool)
	ps := store.NewProjectStore(pool)

	ctx := context.Background()
	project, _ := ps.Create(ctx, uniqueKey("wh-get"), "WH Get", "")
	created, _ := ws.Create(ctx, project.ID, "Get Hook", "https://c.com/hook", "whsec_c", []string{"flag.created"})

	got, err := ws.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("ID = %q, want %q", got.ID, created.ID)
	}
}

func TestWebhookStore_Update(t *testing.T) {
	pool := testPool(t)
	ws := store.NewWebhookStore(pool)
	ps := store.NewProjectStore(pool)

	ctx := context.Background()
	project, _ := ps.Create(ctx, uniqueKey("wh-upd"), "WH Upd", "")
	created, _ := ws.Create(ctx, project.ID, "Old Name", "https://old.com/hook", "whsec_old", []string{"flag.created"})

	updated, err := ws.Update(ctx, created.ID, "New Name", "https://new.com/hook", []string{"flag.updated"}, false)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != "New Name" {
		t.Errorf("Name = %q, want %q", updated.Name, "New Name")
	}
	if updated.Enabled != false {
		t.Errorf("Enabled = %v, want false", updated.Enabled)
	}
}

func TestWebhookStore_Delete(t *testing.T) {
	pool := testPool(t)
	ws := store.NewWebhookStore(pool)
	ps := store.NewProjectStore(pool)

	ctx := context.Background()
	project, _ := ps.Create(ctx, uniqueKey("wh-del"), "WH Del", "")
	created, _ := ws.Create(ctx, project.ID, "Del Hook", "https://del.com/hook", "whsec_del", []string{"flag.created"})

	if err := ws.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := ws.GetByID(ctx, created.ID)
	if err == nil {
		t.Fatal("expected error after delete, got nil")
	}
}

func TestWebhookStore_ListEnabledByProject(t *testing.T) {
	pool := testPool(t)
	ws := store.NewWebhookStore(pool)
	ps := store.NewProjectStore(pool)

	ctx := context.Background()
	project, _ := ps.Create(ctx, uniqueKey("wh-enabled"), "WH Enabled", "")
	ws.Create(ctx, project.ID, "Enabled", "https://a.com/hook", "whsec_a", []string{"flag.created"})
	created2, _ := ws.Create(ctx, project.ID, "Will Disable", "https://b.com/hook", "whsec_b", []string{"flag.created"})
	ws.Update(ctx, created2.ID, created2.Name, created2.URL, created2.EventTypes, false)

	hooks, err := ws.ListEnabledByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListEnabledByProject: %v", err)
	}
	if len(hooks) != 1 {
		t.Errorf("len = %d, want 1 (only enabled)", len(hooks))
	}
}
