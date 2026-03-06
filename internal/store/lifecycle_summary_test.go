package store_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

func TestFlagStore_LifecycleSummary(t *testing.T) {
	pool := testPool(t)
	fs := store.NewFlagStore(pool)
	ps := store.NewProjectStore(pool)
	es := store.NewEnvironmentStore(pool)

	ctx := context.Background()

	project, err := ps.Create(ctx, uniqueKey("summary"), "Summary Test", "")
	if err != nil {
		t.Fatal(err)
	}

	// Need an environment for flag creation
	_, err = es.Create(ctx, project.ID, "dev", "Development")
	if err != nil {
		t.Fatal(err)
	}

	// Create flags in different lifecycle states
	_, err = fs.Create(ctx, project.ID, "flag-active", "Active Flag", "", model.ValueTypeBoolean, model.FlagTypeRelease, json.RawMessage(`false`), []string{}, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	f2, err := fs.Create(ctx, project.ID, "flag-stale", "Stale Flag", "", model.ValueTypeBoolean, model.FlagTypeRelease, json.RawMessage(`false`), []string{}, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = fs.SetLifecycleStatus(ctx, f2.ID, "stale")
	if err != nil {
		t.Fatal(err)
	}

	summary, err := fs.LifecycleSummary(ctx, project.ID)
	if err != nil {
		t.Fatalf("LifecycleSummary failed: %v", err)
	}

	if summary.Active != 1 {
		t.Errorf("expected 1 active, got %d", summary.Active)
	}
	if summary.Stale != 1 {
		t.Errorf("expected 1 stale, got %d", summary.Stale)
	}
	if summary.PotentiallyStale != 0 {
		t.Errorf("expected 0 potentially_stale, got %d", summary.PotentiallyStale)
	}
	if summary.Archived != 0 {
		t.Errorf("expected 0 archived, got %d", summary.Archived)
	}
	// Health = 1/(1+1) * 100 = 50
	if summary.HealthScore < 49 || summary.HealthScore > 51 {
		t.Errorf("expected health_score ~50, got %f", summary.HealthScore)
	}
}
