package evaluation_test

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/togglerino/togglerino/internal/evaluation"
	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

func TestCache_SetAndGetFlags(t *testing.T) {
	c := evaluation.NewCache()
	flags := map[string]evaluation.FlagData{
		"dark-mode": {
			Flag:   model.Flag{Key: "dark-mode", ValueType: model.ValueTypeBoolean},
			Config: model.FlagEnvironmentConfig{Enabled: true, DefaultVariant: "on"},
		},
	}
	c.Set("web-app", "production", flags)

	got := c.GetFlags("web-app", "production")
	if len(got) != 1 {
		t.Fatalf("expected 1 flag, got %d", len(got))
	}
	if _, ok := got["dark-mode"]; !ok {
		t.Error("expected dark-mode flag")
	}
}

func TestCache_GetFlag(t *testing.T) {
	c := evaluation.NewCache()
	flags := map[string]evaluation.FlagData{
		"dark-mode": {
			Flag:   model.Flag{Key: "dark-mode"},
			Config: model.FlagEnvironmentConfig{Enabled: true},
		},
	}
	c.Set("web-app", "production", flags)

	fd, ok := c.GetFlag("web-app", "production", "dark-mode")
	if !ok {
		t.Fatal("expected to find flag")
	}
	if fd.Flag.Key != "dark-mode" {
		t.Errorf("got key %q, want dark-mode", fd.Flag.Key)
	}

	_, ok = c.GetFlag("web-app", "production", "nonexistent")
	if ok {
		t.Error("expected not to find nonexistent flag")
	}

	_, ok = c.GetFlag("nonexistent", "production", "dark-mode")
	if ok {
		t.Error("expected not to find flag for nonexistent project")
	}
}

func TestCache_GetFlags_Empty(t *testing.T) {
	c := evaluation.NewCache()
	got := c.GetFlags("no-project", "no-env")
	if got != nil {
		t.Errorf("expected nil for unknown project/env, got %v", got)
	}
}

func TestCache_SetOverwrites(t *testing.T) {
	c := evaluation.NewCache()

	c.Set("proj", "env", map[string]evaluation.FlagData{
		"flag-a": {Flag: model.Flag{Key: "flag-a"}},
		"flag-b": {Flag: model.Flag{Key: "flag-b"}},
	})

	// Overwrite with a different set of flags.
	c.Set("proj", "env", map[string]evaluation.FlagData{
		"flag-c": {Flag: model.Flag{Key: "flag-c"}},
	})

	got := c.GetFlags("proj", "env")
	if len(got) != 1 {
		t.Fatalf("expected 1 flag after overwrite, got %d", len(got))
	}
	if _, ok := got["flag-c"]; !ok {
		t.Error("expected flag-c after overwrite")
	}
	if _, ok := got["flag-a"]; ok {
		t.Error("flag-a should not exist after overwrite")
	}
}

func TestCache_MultipleProjectsEnvironments(t *testing.T) {
	c := evaluation.NewCache()

	c.Set("proj-1", "staging", map[string]evaluation.FlagData{
		"flag-x": {Flag: model.Flag{Key: "flag-x"}},
	})
	c.Set("proj-1", "production", map[string]evaluation.FlagData{
		"flag-y": {Flag: model.Flag{Key: "flag-y"}},
	})
	c.Set("proj-2", "staging", map[string]evaluation.FlagData{
		"flag-z": {Flag: model.Flag{Key: "flag-z"}},
	})

	// Verify isolation between project/environment combos.
	if _, ok := c.GetFlag("proj-1", "staging", "flag-x"); !ok {
		t.Error("expected flag-x in proj-1/staging")
	}
	if _, ok := c.GetFlag("proj-1", "staging", "flag-y"); ok {
		t.Error("flag-y should not be in proj-1/staging")
	}
	if _, ok := c.GetFlag("proj-1", "production", "flag-y"); !ok {
		t.Error("expected flag-y in proj-1/production")
	}
	if _, ok := c.GetFlag("proj-2", "staging", "flag-z"); !ok {
		t.Error("expected flag-z in proj-2/staging")
	}
}

func TestCache_ConcurrentAccess(t *testing.T) {
	c := evaluation.NewCache()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			c.Set("proj", "env", map[string]evaluation.FlagData{
				fmt.Sprintf("flag-%d", i): {},
			})
		}(i)
		go func() {
			defer wg.Done()
			c.GetFlags("proj", "env")
		}()
	}
	wg.Wait()
}

func TestCache_ConcurrentReadWrite(t *testing.T) {
	c := evaluation.NewCache()
	// Pre-populate so reads have data.
	c.Set("proj", "env", map[string]evaluation.FlagData{
		"seed-flag": {Flag: model.Flag{Key: "seed-flag"}},
	})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(3)
		go func(i int) {
			defer wg.Done()
			c.Set("proj", "env", map[string]evaluation.FlagData{
				fmt.Sprintf("flag-%d", i): {Flag: model.Flag{Key: fmt.Sprintf("flag-%d", i)}},
			})
		}(i)
		go func() {
			defer wg.Done()
			c.GetFlag("proj", "env", "seed-flag")
		}()
		go func() {
			defer wg.Done()
			c.GetFlags("proj", "env")
		}()
	}
	wg.Wait()
}

func TestCache_Overrides(t *testing.T) {
	c := evaluation.NewCache()

	// Set an override
	c.SetOverride("myproject", "production", "app-user-1", "feature-x", json.RawMessage(`true`), nil)

	// Get override — should find it
	val, ok := c.GetOverride("myproject", "production", "app-user-1", "feature-x")
	if !ok {
		t.Fatal("expected to find override")
	}
	if string(val) != "true" {
		t.Fatalf("expected true, got %s", string(val))
	}

	// Get override for different user — should not find it
	_, ok = c.GetOverride("myproject", "production", "other-user", "feature-x")
	if ok {
		t.Fatal("expected no override for other user")
	}

	// Delete override
	c.DeleteOverride("myproject", "production", "app-user-1", "feature-x")
	_, ok = c.GetOverride("myproject", "production", "app-user-1", "feature-x")
	if ok {
		t.Fatal("expected override to be deleted")
	}
}

func TestCache_OverrideExpiry(t *testing.T) {
	c := evaluation.NewCache()

	past := time.Now().Add(-1 * time.Hour)
	c.SetOverride("proj", "dev", "user-1", "flag-a", json.RawMessage(`true`), &past)

	// Expired override should not be returned
	_, ok := c.GetOverride("proj", "dev", "user-1", "flag-a")
	if ok {
		t.Fatal("expected expired override to not be found")
	}
}

func TestCache_DeleteOverridesForUser(t *testing.T) {
	c := evaluation.NewCache()

	c.SetOverride("proj", "dev", "user-1", "flag-a", json.RawMessage(`true`), nil)
	c.SetOverride("proj", "dev", "user-1", "flag-b", json.RawMessage(`false`), nil)
	c.SetOverride("proj", "dev", "user-2", "flag-a", json.RawMessage(`true`), nil)

	c.DeleteOverridesForUser("proj", "dev", "user-1")

	_, ok := c.GetOverride("proj", "dev", "user-1", "flag-a")
	if ok {
		t.Fatal("expected user-1 flag-a to be deleted")
	}
	_, ok = c.GetOverride("proj", "dev", "user-1", "flag-b")
	if ok {
		t.Fatal("expected user-1 flag-b to be deleted")
	}
	// user-2 should be unaffected
	_, ok = c.GetOverride("proj", "dev", "user-2", "flag-a")
	if !ok {
		t.Fatal("expected user-2 flag-a to still exist")
	}
}

func TestCache_LoadOverrides(t *testing.T) {
	c := evaluation.NewCache()

	entries := []store.OverrideCacheEntry{
		{ProjectKey: "proj", EnvironmentKey: "dev", FlagKey: "flag-a", AppUserID: "user-1", Value: json.RawMessage(`true`)},
		{ProjectKey: "proj", EnvironmentKey: "prod", FlagKey: "flag-b", AppUserID: "user-2", Value: json.RawMessage(`42`)},
	}
	c.LoadOverrides(entries)

	val, ok := c.GetOverride("proj", "dev", "user-1", "flag-a")
	if !ok {
		t.Fatal("expected override after LoadOverrides")
	}
	if string(val) != "true" {
		t.Fatalf("expected true, got %s", string(val))
	}

	val, ok = c.GetOverride("proj", "prod", "user-2", "flag-b")
	if !ok {
		t.Fatal("expected override after LoadOverrides")
	}
	if string(val) != "42" {
		t.Fatalf("expected 42, got %s", string(val))
	}
}
