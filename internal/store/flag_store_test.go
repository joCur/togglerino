package store_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

func TestFlagStore_Create(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	es := store.NewEnvironmentStore(pool)
	fs := store.NewFlagStore(pool)
	ctx := context.Background()

	// Create project and environments
	projKey := uniqueKey("flagcreate")
	project, err := ps.Create(ctx, projKey, "Flag Create Project", "test")
	if err != nil {
		t.Fatalf("creating project: %v", err)
	}

	env1, err := es.Create(ctx, project.ID, "development", "Development")
	if err != nil {
		t.Fatalf("creating env1: %v", err)
	}

	env2, err := es.Create(ctx, project.ID, "production", "Production")
	if err != nil {
		t.Fatalf("creating env2: %v", err)
	}

	defaultValue := json.RawMessage(`false`)
	flag, err := fs.Create(ctx, project.ID, "dark-mode", "Dark Mode", "Toggle dark mode", model.FlagTypeBoolean, defaultValue, []string{"ui", "frontend"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if flag.ID == "" {
		t.Error("expected non-empty ID")
	}
	if flag.ProjectID != project.ID {
		t.Errorf("ProjectID: got %q, want %q", flag.ProjectID, project.ID)
	}
	if flag.Key != "dark-mode" {
		t.Errorf("Key: got %q, want %q", flag.Key, "dark-mode")
	}
	if flag.Name != "Dark Mode" {
		t.Errorf("Name: got %q, want %q", flag.Name, "Dark Mode")
	}
	if flag.Description != "Toggle dark mode" {
		t.Errorf("Description: got %q, want %q", flag.Description, "Toggle dark mode")
	}
	if flag.FlagType != model.FlagTypeBoolean {
		t.Errorf("FlagType: got %q, want %q", flag.FlagType, model.FlagTypeBoolean)
	}
	if len(flag.Tags) != 2 {
		t.Errorf("Tags length: got %d, want 2", len(flag.Tags))
	}
	if flag.Archived {
		t.Error("expected Archived to be false")
	}
	if flag.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
	if flag.UpdatedAt.IsZero() {
		t.Error("expected non-zero UpdatedAt")
	}

	// Verify environment configs were created for both environments
	configs, err := fs.GetAllEnvironmentConfigs(ctx, flag.ID)
	if err != nil {
		t.Fatalf("GetAllEnvironmentConfigs: %v", err)
	}

	if len(configs) != 2 {
		t.Fatalf("expected 2 environment configs, got %d", len(configs))
	}

	// Verify each config is disabled by default
	envIDs := map[string]bool{env1.ID: false, env2.ID: false}
	for _, cfg := range configs {
		if cfg.FlagID != flag.ID {
			t.Errorf("config FlagID: got %q, want %q", cfg.FlagID, flag.ID)
		}
		if cfg.Enabled {
			t.Errorf("expected config for env %q to be disabled", cfg.EnvironmentID)
		}
		envIDs[cfg.EnvironmentID] = true
	}
	for envID, found := range envIDs {
		if !found {
			t.Errorf("expected config for environment %q", envID)
		}
	}
}

func TestFlagStore_ListByProject(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	es := store.NewEnvironmentStore(pool)
	fs := store.NewFlagStore(pool)
	ctx := context.Background()

	projKey := uniqueKey("flaglist")
	project, err := ps.Create(ctx, projKey, "Flag List Project", "test")
	if err != nil {
		t.Fatalf("creating project: %v", err)
	}

	// Create at least one environment so Create succeeds
	_, err = es.Create(ctx, project.ID, "dev", "Development")
	if err != nil {
		t.Fatalf("creating env: %v", err)
	}

	_, err = fs.Create(ctx, project.ID, "flag-a", "Flag A", "first flag", model.FlagTypeBoolean, json.RawMessage(`false`), []string{"ui"})
	if err != nil {
		t.Fatalf("Create flag-a: %v", err)
	}

	_, err = fs.Create(ctx, project.ID, "flag-b", "Flag B", "second flag", model.FlagTypeString, json.RawMessage(`"default"`), []string{"backend"})
	if err != nil {
		t.Fatalf("Create flag-b: %v", err)
	}

	_, err = fs.Create(ctx, project.ID, "flag-c", "Dark Theme", "third flag", model.FlagTypeBoolean, json.RawMessage(`true`), []string{"ui", "frontend"})
	if err != nil {
		t.Fatalf("Create flag-c: %v", err)
	}

	// Basic list — should return all 3
	flags, err := fs.ListByProject(ctx, project.ID, "", "")
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	if len(flags) != 3 {
		t.Fatalf("expected 3 flags, got %d", len(flags))
	}

	// Filter by tag "ui" — should return flag-a and flag-c
	flags, err = fs.ListByProject(ctx, project.ID, "ui", "")
	if err != nil {
		t.Fatalf("ListByProject with tag: %v", err)
	}
	if len(flags) != 2 {
		t.Fatalf("expected 2 flags with tag 'ui', got %d", len(flags))
	}

	// Filter by tag "backend" — should return flag-b
	flags, err = fs.ListByProject(ctx, project.ID, "backend", "")
	if err != nil {
		t.Fatalf("ListByProject with tag 'backend': %v", err)
	}
	if len(flags) != 1 {
		t.Fatalf("expected 1 flag with tag 'backend', got %d", len(flags))
	}
	if flags[0].Key != "flag-b" {
		t.Errorf("expected flag-b, got %q", flags[0].Key)
	}

	// Search by name "Dark" — should return flag-c
	flags, err = fs.ListByProject(ctx, project.ID, "", "Dark")
	if err != nil {
		t.Fatalf("ListByProject with search 'Dark': %v", err)
	}
	if len(flags) != 1 {
		t.Fatalf("expected 1 flag matching search 'Dark', got %d", len(flags))
	}
	if flags[0].Key != "flag-c" {
		t.Errorf("expected flag-c, got %q", flags[0].Key)
	}

	// Search by key "flag-a" — should match flag-a
	flags, err = fs.ListByProject(ctx, project.ID, "", "flag-a")
	if err != nil {
		t.Fatalf("ListByProject with search 'flag-a': %v", err)
	}
	if len(flags) != 1 {
		t.Fatalf("expected 1 flag matching search 'flag-a', got %d", len(flags))
	}
	if flags[0].Key != "flag-a" {
		t.Errorf("expected flag-a, got %q", flags[0].Key)
	}
}

func TestFlagStore_FindByKey(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	es := store.NewEnvironmentStore(pool)
	fs := store.NewFlagStore(pool)
	ctx := context.Background()

	projKey := uniqueKey("flagfind")
	project, err := ps.Create(ctx, projKey, "Flag Find Project", "test")
	if err != nil {
		t.Fatalf("creating project: %v", err)
	}

	_, err = es.Create(ctx, project.ID, "dev", "Development")
	if err != nil {
		t.Fatalf("creating env: %v", err)
	}

	created, err := fs.Create(ctx, project.ID, "find-me", "Find Me", "findable flag", model.FlagTypeBoolean, json.RawMessage(`false`), []string{"test"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	found, err := fs.FindByKey(ctx, project.ID, "find-me")
	if err != nil {
		t.Fatalf("FindByKey: %v", err)
	}

	if found.ID != created.ID {
		t.Errorf("ID: got %q, want %q", found.ID, created.ID)
	}
	if found.Key != "find-me" {
		t.Errorf("Key: got %q, want %q", found.Key, "find-me")
	}
	if found.Name != "Find Me" {
		t.Errorf("Name: got %q, want %q", found.Name, "Find Me")
	}
}

func TestFlagStore_FindByKey_NotFound(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	fs := store.NewFlagStore(pool)
	ctx := context.Background()

	projKey := uniqueKey("flagfindnf")
	project, err := ps.Create(ctx, projKey, "Not Found Project", "test")
	if err != nil {
		t.Fatalf("creating project: %v", err)
	}

	_, err = fs.FindByKey(ctx, project.ID, "nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent flag key, got nil")
	}
}

func TestFlagStore_Update(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	es := store.NewEnvironmentStore(pool)
	fs := store.NewFlagStore(pool)
	ctx := context.Background()

	projKey := uniqueKey("flagupdate")
	project, err := ps.Create(ctx, projKey, "Flag Update Project", "test")
	if err != nil {
		t.Fatalf("creating project: %v", err)
	}

	_, err = es.Create(ctx, project.ID, "dev", "Development")
	if err != nil {
		t.Fatalf("creating env: %v", err)
	}

	created, err := fs.Create(ctx, project.ID, "update-me", "Old Name", "old description", model.FlagTypeBoolean, json.RawMessage(`false`), []string{"old"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated, err := fs.Update(ctx, created.ID, "New Name", "new description", []string{"new", "updated"})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	if updated.ID != created.ID {
		t.Errorf("ID should not change: got %q, want %q", updated.ID, created.ID)
	}
	if updated.Key != "update-me" {
		t.Errorf("Key should not change: got %q, want %q", updated.Key, "update-me")
	}
	if updated.Name != "New Name" {
		t.Errorf("Name: got %q, want %q", updated.Name, "New Name")
	}
	if updated.Description != "new description" {
		t.Errorf("Description: got %q, want %q", updated.Description, "new description")
	}
	if len(updated.Tags) != 2 {
		t.Errorf("Tags length: got %d, want 2", len(updated.Tags))
	}
}

func TestFlagStore_Delete(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	es := store.NewEnvironmentStore(pool)
	fs := store.NewFlagStore(pool)
	ctx := context.Background()

	projKey := uniqueKey("flagdelete")
	project, err := ps.Create(ctx, projKey, "Flag Delete Project", "test")
	if err != nil {
		t.Fatalf("creating project: %v", err)
	}

	_, err = es.Create(ctx, project.ID, "dev", "Development")
	if err != nil {
		t.Fatalf("creating env: %v", err)
	}

	flag, err := fs.Create(ctx, project.ID, "delete-me", "Delete Me", "to be deleted", model.FlagTypeBoolean, json.RawMessage(`false`), []string{})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	err = fs.Delete(ctx, flag.ID)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Verify it's gone
	_, err = fs.FindByKey(ctx, project.ID, "delete-me")
	if err == nil {
		t.Fatal("expected error after deletion, got nil")
	}

	// Verify environment configs are also gone (cascade)
	configs, err := fs.GetAllEnvironmentConfigs(ctx, flag.ID)
	if err != nil {
		t.Fatalf("GetAllEnvironmentConfigs after delete: %v", err)
	}
	if len(configs) != 0 {
		t.Errorf("expected 0 configs after flag deletion, got %d", len(configs))
	}
}

func TestFlagStore_GetEnvironmentConfig(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	es := store.NewEnvironmentStore(pool)
	fs := store.NewFlagStore(pool)
	ctx := context.Background()

	projKey := uniqueKey("flagenvcfg")
	project, err := ps.Create(ctx, projKey, "Env Config Project", "test")
	if err != nil {
		t.Fatalf("creating project: %v", err)
	}

	env, err := es.Create(ctx, project.ID, "staging", "Staging")
	if err != nil {
		t.Fatalf("creating env: %v", err)
	}

	flag, err := fs.Create(ctx, project.ID, "env-cfg-flag", "Env Config Flag", "test", model.FlagTypeBoolean, json.RawMessage(`false`), []string{})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	cfg, err := fs.GetEnvironmentConfig(ctx, flag.ID, env.ID)
	if err != nil {
		t.Fatalf("GetEnvironmentConfig: %v", err)
	}

	if cfg.ID == "" {
		t.Error("expected non-empty ID")
	}
	if cfg.FlagID != flag.ID {
		t.Errorf("FlagID: got %q, want %q", cfg.FlagID, flag.ID)
	}
	if cfg.EnvironmentID != env.ID {
		t.Errorf("EnvironmentID: got %q, want %q", cfg.EnvironmentID, env.ID)
	}
	if cfg.Enabled {
		t.Error("expected Enabled to be false by default")
	}
	if cfg.Variants == nil {
		t.Error("expected non-nil Variants")
	}
	if cfg.TargetingRules == nil {
		t.Error("expected non-nil TargetingRules")
	}
}

func TestFlagStore_GetAllEnvironmentConfigs(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	es := store.NewEnvironmentStore(pool)
	fs := store.NewFlagStore(pool)
	ctx := context.Background()

	projKey := uniqueKey("flagallcfg")
	project, err := ps.Create(ctx, projKey, "All Configs Project", "test")
	if err != nil {
		t.Fatalf("creating project: %v", err)
	}

	_, err = es.Create(ctx, project.ID, "dev", "Development")
	if err != nil {
		t.Fatalf("creating env1: %v", err)
	}

	_, err = es.Create(ctx, project.ID, "staging", "Staging")
	if err != nil {
		t.Fatalf("creating env2: %v", err)
	}

	_, err = es.Create(ctx, project.ID, "prod", "Production")
	if err != nil {
		t.Fatalf("creating env3: %v", err)
	}

	flag, err := fs.Create(ctx, project.ID, "all-cfg-flag", "All Config Flag", "test", model.FlagTypeBoolean, json.RawMessage(`false`), []string{})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	configs, err := fs.GetAllEnvironmentConfigs(ctx, flag.ID)
	if err != nil {
		t.Fatalf("GetAllEnvironmentConfigs: %v", err)
	}

	if len(configs) != 3 {
		t.Fatalf("expected 3 environment configs, got %d", len(configs))
	}
}

func TestFlagStore_SetArchived(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	es := store.NewEnvironmentStore(pool)
	fs := store.NewFlagStore(pool)
	ctx := context.Background()

	projKey := uniqueKey("flagarchive")
	project, err := ps.Create(ctx, projKey, "Archive Project", "test")
	if err != nil {
		t.Fatalf("creating project: %v", err)
	}

	_, err = es.Create(ctx, project.ID, "dev", "Development")
	if err != nil {
		t.Fatalf("creating env: %v", err)
	}

	flag, err := fs.Create(ctx, project.ID, "archive-me", "Archive Me", "test", model.FlagTypeBoolean, json.RawMessage(`false`), []string{})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if flag.Archived {
		t.Fatal("expected newly created flag to not be archived")
	}

	// Archive
	archived, err := fs.SetArchived(ctx, flag.ID, true)
	if err != nil {
		t.Fatalf("SetArchived(true): %v", err)
	}
	if !archived.Archived {
		t.Error("expected Archived to be true")
	}
	if archived.ID != flag.ID {
		t.Errorf("ID: got %q, want %q", archived.ID, flag.ID)
	}

	// Unarchive
	unarchived, err := fs.SetArchived(ctx, flag.ID, false)
	if err != nil {
		t.Fatalf("SetArchived(false): %v", err)
	}
	if unarchived.Archived {
		t.Error("expected Archived to be false")
	}
}

func TestFlagStore_UpdateEnvironmentConfig(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	es := store.NewEnvironmentStore(pool)
	fs := store.NewFlagStore(pool)
	ctx := context.Background()

	projKey := uniqueKey("flagupdcfg")
	project, err := ps.Create(ctx, projKey, "Update Config Project", "test")
	if err != nil {
		t.Fatalf("creating project: %v", err)
	}

	env, err := es.Create(ctx, project.ID, "production", "Production")
	if err != nil {
		t.Fatalf("creating env: %v", err)
	}

	flag, err := fs.Create(ctx, project.ID, "upd-cfg-flag", "Update Config Flag", "test", model.FlagTypeBoolean, json.RawMessage(`false`), []string{})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Update the config: enable flag, set variants, add targeting rules
	variants := json.RawMessage(`[{"key":"on","value":true},{"key":"off","value":false}]`)
	rules := json.RawMessage(`[{"conditions":[{"attribute":"country","operator":"equals","value":"US"}],"variant":"on"}]`)

	cfg, err := fs.UpdateEnvironmentConfig(ctx, flag.ID, env.ID, true, "on", variants, rules)
	if err != nil {
		t.Fatalf("UpdateEnvironmentConfig: %v", err)
	}

	if !cfg.Enabled {
		t.Error("expected Enabled to be true")
	}
	if cfg.DefaultVariant != "on" {
		t.Errorf("DefaultVariant: got %q, want %q", cfg.DefaultVariant, "on")
	}
	if len(cfg.Variants) != 2 {
		t.Errorf("Variants length: got %d, want 2", len(cfg.Variants))
	}
	if len(cfg.TargetingRules) != 1 {
		t.Errorf("TargetingRules length: got %d, want 1", len(cfg.TargetingRules))
	}

	// Verify the targeting rule details
	if len(cfg.TargetingRules) > 0 {
		rule := cfg.TargetingRules[0]
		if rule.Variant != "on" {
			t.Errorf("rule Variant: got %q, want %q", rule.Variant, "on")
		}
		if len(rule.Conditions) != 1 {
			t.Errorf("rule Conditions length: got %d, want 1", len(rule.Conditions))
		}
		if len(rule.Conditions) > 0 {
			cond := rule.Conditions[0]
			if cond.Attribute != "country" {
				t.Errorf("condition Attribute: got %q, want %q", cond.Attribute, "country")
			}
			if cond.Operator != "equals" {
				t.Errorf("condition Operator: got %q, want %q", cond.Operator, "equals")
			}
		}
	}

	// Verify we can read it back
	readCfg, err := fs.GetEnvironmentConfig(ctx, flag.ID, env.ID)
	if err != nil {
		t.Fatalf("GetEnvironmentConfig after update: %v", err)
	}
	if !readCfg.Enabled {
		t.Error("expected Enabled to be true after re-read")
	}
	if readCfg.DefaultVariant != "on" {
		t.Errorf("DefaultVariant after re-read: got %q, want %q", readCfg.DefaultVariant, "on")
	}
	if len(readCfg.Variants) != 2 {
		t.Errorf("Variants length after re-read: got %d, want 2", len(readCfg.Variants))
	}
}
