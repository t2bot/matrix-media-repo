package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRateLimitRequestsEnabledConfiguration(t *testing.T) {
	t.Run("defaults to enabled", func(t *testing.T) {
		cfg := NewDefaultMainConfig()

		if !cfg.RateLimit.RequestsEnabled {
			t.Fatal("expected request rate limit to be enabled by default")
		}
	})

	t.Run("legacy configuration keeps it enabled", func(t *testing.T) {
		cfg := NewDefaultMainConfig()
		overrides := map[string]interface{}{
			"rateLimit": map[string]interface{}{
				"requestsPerSecond": 20,
			},
		}

		if err := mapToObjYaml(overrides, &cfg); err != nil {
			t.Fatalf("failed to apply configuration: %v", err)
		}
		if !cfg.RateLimit.RequestsEnabled {
			t.Fatal("expected request rate limit to remain enabled when requestsEnabled is omitted")
		}
	})

	t.Run("can be disabled independently", func(t *testing.T) {
		cfg := NewDefaultMainConfig()
		overrides := map[string]interface{}{
			"rateLimit": map[string]interface{}{
				"requestsEnabled": false,
			},
		}

		if err := mapToObjYaml(overrides, &cfg); err != nil {
			t.Fatalf("failed to apply configuration: %v", err)
		}
		if cfg.RateLimit.RequestsEnabled {
			t.Fatal("expected request rate limit to be disabled")
		}
		if !cfg.RateLimit.Enabled {
			t.Fatal("expected byte-based rate limit buckets to remain enabled")
		}
	})
}

func TestLoadFromPathForTestsReloadsConfig(t *testing.T) {
	originalPath := Path
	originalInstance := instance
	originalDomains := domains
	originalSingletonLock := singletonLock
	t.Cleanup(func() {
		Path = originalPath
		instance = originalInstance
		domains = originalDomains
		singletonLock = originalSingletonLock
	})

	dir := t.TempDir()
	firstPath := filepath.Join(dir, "first.yaml")
	secondPath := filepath.Join(dir, "second.yaml")
	if err := os.WriteFile(
		firstPath,
		[]byte("database:\n  postgres: postgres://first\n"),
		0600,
	); err != nil {
		t.Fatalf("failed to write first config: %v", err)
	}
	if err := os.WriteFile(
		secondPath,
		[]byte("database:\n  postgres: postgres://second\n"),
		0600,
	); err != nil {
		t.Fatalf("failed to write second config: %v", err)
	}

	if err := LoadFromPathForTests(firstPath); err != nil {
		t.Fatalf("failed to load first config: %v", err)
	}
	if got := Get().Database.Postgres; got != "postgres://first" {
		t.Fatalf("expected first database, got %q", got)
	}

	if err := LoadFromPathForTests(secondPath); err != nil {
		t.Fatalf("failed to load second config: %v", err)
	}
	if got := Get().Database.Postgres; got != "postgres://second" {
		t.Fatalf("expected second database, got %q", got)
	}
}
