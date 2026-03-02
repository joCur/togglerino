package model

import "testing"

func TestResolveEnvironmentDefaults_NilReceiver(t *testing.T) {
	var ps *ProjectSettings
	result := ps.ResolveEnvironmentDefaults([]string{"development", "staging", "production"}, nil)

	if !result["development"] {
		t.Error("expected development=true from hardcoded fallback")
	}
	if result["staging"] {
		t.Error("expected staging=false from hardcoded fallback")
	}
	if result["production"] {
		t.Error("expected production=false from hardcoded fallback")
	}
}

func TestResolveEnvironmentDefaults_HardcodedFallback(t *testing.T) {
	ps := &ProjectSettings{}
	result := ps.ResolveEnvironmentDefaults([]string{"development", "staging", "production", "custom-env"}, nil)

	if !result["development"] {
		t.Error("expected development=true")
	}
	if result["staging"] {
		t.Error("expected staging=false")
	}
	if result["production"] {
		t.Error("expected production=false")
	}
	if result["custom-env"] {
		t.Error("expected custom-env=false (unknown env)")
	}
}

func TestResolveEnvironmentDefaults_ProjectOverride(t *testing.T) {
	ps := &ProjectSettings{
		EnvironmentDefaults: map[string]EnvironmentDefault{
			"development": {Enabled: false},
			"staging":     {Enabled: true},
		},
	}
	result := ps.ResolveEnvironmentDefaults([]string{"development", "staging", "production"}, nil)

	if result["development"] {
		t.Error("expected development=false (project override)")
	}
	if !result["staging"] {
		t.Error("expected staging=true (project override)")
	}
	if result["production"] {
		t.Error("expected production=false (hardcoded fallback, no override)")
	}
}

func TestResolveEnvironmentDefaults_RequestOverride(t *testing.T) {
	ps := &ProjectSettings{
		EnvironmentDefaults: map[string]EnvironmentDefault{
			"development": {Enabled: true},
			"staging":     {Enabled: false},
		},
	}
	overrides := map[string]EnvironmentDefault{
		"staging":    {Enabled: true},
		"production": {Enabled: true},
	}
	result := ps.ResolveEnvironmentDefaults([]string{"development", "staging", "production"}, overrides)

	if !result["development"] {
		t.Error("expected development=true (project setting, no override)")
	}
	if !result["staging"] {
		t.Error("expected staging=true (request override)")
	}
	if !result["production"] {
		t.Error("expected production=true (request override)")
	}
}

func TestResolveEnvironmentDefaults_ThreeLayerPrecedence(t *testing.T) {
	// hardcoded: development=true
	// project: development=false
	// override: development=true
	// Should resolve to true (override wins)
	ps := &ProjectSettings{
		EnvironmentDefaults: map[string]EnvironmentDefault{
			"development": {Enabled: false},
		},
	}
	overrides := map[string]EnvironmentDefault{
		"development": {Enabled: true},
	}
	result := ps.ResolveEnvironmentDefaults([]string{"development"}, overrides)

	if !result["development"] {
		t.Error("expected development=true (request override should win over project setting)")
	}
}

func TestDefaultEnvironmentDefaults(t *testing.T) {
	defaults := DefaultEnvironmentDefaults()

	if !defaults["development"].Enabled {
		t.Error("expected development=true")
	}
	if defaults["staging"].Enabled {
		t.Error("expected staging=false")
	}
	if defaults["production"].Enabled {
		t.Error("expected production=false")
	}
	if len(defaults) != 3 {
		t.Errorf("expected 3 defaults, got %d", len(defaults))
	}
}
