package config

import "testing"

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
