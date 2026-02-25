package evaluation_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/togglerino/togglerino/internal/evaluation"
	"github.com/togglerino/togglerino/internal/model"
)

func TestCache_SetAndGetFlags(t *testing.T) {
	c := evaluation.NewCache()
	flags := map[string]evaluation.FlagData{
		"dark-mode": {
			Flag:   model.Flag{Key: "dark-mode", FlagType: model.FlagTypeBoolean},
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
