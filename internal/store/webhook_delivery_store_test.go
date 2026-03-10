package store_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/togglerino/togglerino/internal/store"
)

func TestWebhookDeliveryStore_Record(t *testing.T) {
	pool := testPool(t)
	ws := store.NewWebhookStore(pool)
	ds := store.NewWebhookDeliveryStore(pool)
	ps := store.NewProjectStore(pool)

	ctx := context.Background()
	project, _ := ps.Create(ctx, uniqueKey("del-rec"), "Del Rec", "")
	wh, _ := ws.Create(ctx, project.ID, "Hook", "https://example.com", "whsec_x", []string{"flag.created"})

	statusCode := 200
	durationMs := 150
	delivery, err := ds.Record(ctx, wh.ID, "flag.created", json.RawMessage(`{"type":"flag.created"}`), &statusCode, nil, nil, 1, true, &durationMs)
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	if delivery.WebhookID != wh.ID {
		t.Errorf("WebhookID = %q, want %q", delivery.WebhookID, wh.ID)
	}
	if !delivery.Success {
		t.Error("Success = false, want true")
	}
}

func TestWebhookDeliveryStore_ListByWebhook(t *testing.T) {
	pool := testPool(t)
	ws := store.NewWebhookStore(pool)
	ds := store.NewWebhookDeliveryStore(pool)
	ps := store.NewProjectStore(pool)

	ctx := context.Background()
	project, _ := ps.Create(ctx, uniqueKey("del-list"), "Del List", "")
	wh, _ := ws.Create(ctx, project.ID, "Hook", "https://example.com", "whsec_x", []string{"flag.created"})

	sc := 200
	dur := 100
	ds.Record(ctx, wh.ID, "flag.created", json.RawMessage(`{}`), &sc, nil, nil, 1, true, &dur)
	ds.Record(ctx, wh.ID, "flag.updated", json.RawMessage(`{}`), &sc, nil, nil, 1, true, &dur)

	deliveries, total, err := ds.ListByWebhook(ctx, wh.ID, 50, 0)
	if err != nil {
		t.Fatalf("ListByWebhook: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(deliveries) != 2 {
		t.Errorf("len = %d, want 2", len(deliveries))
	}
}

func TestWebhookDeliveryStore_DeleteOlderThan(t *testing.T) {
	pool := testPool(t)
	ws := store.NewWebhookStore(pool)
	ds := store.NewWebhookDeliveryStore(pool)
	ps := store.NewProjectStore(pool)

	ctx := context.Background()
	project, _ := ps.Create(ctx, uniqueKey("del-clean"), "Del Clean", "")
	wh, _ := ws.Create(ctx, project.ID, "Hook", "https://example.com", "whsec_x", []string{"flag.created"})

	sc := 200
	dur := 100
	ds.Record(ctx, wh.ID, "flag.created", json.RawMessage(`{}`), &sc, nil, nil, 1, true, &dur)

	deleted, err := ds.DeleteOlderThan(ctx, 0)
	if err != nil {
		t.Fatalf("DeleteOlderThan: %v", err)
	}
	if deleted == 0 {
		t.Error("expected at least 1 deleted row")
	}
}
