package config

import "testing"

func TestRuntimeConfigApplyTier1(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()
	cfg.Domains = "test.txt"
	rc := NewRuntimeConfig(*cfg)

	concurrency := 512
	patch := ConfigPatch{
		Crawling: &CrawlingPatch{
			Concurrency: &concurrency,
		},
	}

	warnings, err := rc.Apply(patch)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if rc.Get().Crawling.Concurrency != 512 {
		t.Errorf("concurrency = %d", rc.Get().Crawling.Concurrency)
	}
}

func TestRuntimeConfigApplyTier2Warning(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()
	cfg.Domains = "test.txt"
	rc := NewRuntimeConfig(*cfg)

	ingest := "https://new-ingest.com"
	patch := ConfigPatch{
		Output: &OutputPatch{
			Ingest: &ingest,
		},
	}

	warnings, err := rc.Apply(patch)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(warnings) == 0 {
		t.Error("expected Tier 2 warning")
	}
	if rc.Get().Output.Ingest != "https://new-ingest.com" {
		t.Error("ingest not updated")
	}
}

func TestRuntimeConfigApplyTier3Rejected(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()
	cfg.Domains = "test.txt"
	rc := NewRuntimeConfig(*cfg)

	addr := "redis:6379"
	patch := ConfigPatch{
		Redis: &RedisPatch{
			Addr: &addr,
		},
	}

	_, err := rc.Apply(patch)
	if err == nil {
		t.Error("expected Tier 3 rejection")
	}
}

func TestRuntimeConfigOnChange(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()
	cfg.Domains = "test.txt"
	rc := NewRuntimeConfig(*cfg)

	called := false
	rc.OnChange(func(old, new_ Config) {
		called = true
		if old.Crawling.Concurrency == new_.Crawling.Concurrency {
			t.Error("old and new should differ")
		}
	})

	concurrency := 512
	rc.Apply(ConfigPatch{Crawling: &CrawlingPatch{Concurrency: &concurrency}})
	if !called {
		t.Error("OnChange not called")
	}
}

func TestRuntimeConfigApplyUA(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()
	cfg.Domains = "test.txt"
	rc := NewRuntimeConfig(*cfg)

	ua := "CustomAgent/2.0"
	_, err := rc.Apply(ConfigPatch{UA: &ua})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if rc.Get().UA != "CustomAgent/2.0" {
		t.Errorf("UA = %q", rc.Get().UA)
	}
}

func TestRuntimeConfigApplyExcludeTLDs(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()
	cfg.Domains = "test.txt"
	rc := NewRuntimeConfig(*cfg)

	tlds := []string{"gov", "mil", "edu"}
	_, err := rc.Apply(ConfigPatch{ExcludeTLDs: &tlds})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	got := rc.Get().ExcludeTLDs
	if len(got) != 3 || got[2] != "edu" {
		t.Errorf("ExcludeTLDs = %v", got)
	}
}
