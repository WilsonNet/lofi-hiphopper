package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultIsValid(t *testing.T) {
	cfg := Default()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("default config invalid: %v", err)
	}
	if cfg.Radios[0].URL != "https://www.youtube.com/watch?v=rFZHOHl-L8A" {
		t.Fatalf("first radio should be the default lofi stream, got %q", cfg.Radios[0].URL)
	}
	if cfg.Volume != 55 || cfg.Poll() != time.Second || !cfg.Autofill {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestLoadOrCreateWritesDefaultsOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.yaml")

	cfg, created, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("LoadOrCreate: %v", err)
	}
	if !created {
		t.Fatal("first load should report created")
	}
	if len(cfg.Radios) != len(DefaultRadios()) {
		t.Fatalf("got %d radios, want %d", len(cfg.Radios), len(DefaultRadios()))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read default config: %v", err)
	}
	if !strings.Contains(string(data), "poll_interval: 1s") {
		t.Fatalf("default config lacks human duration:\n%s", data)
	}

	_, created, err = LoadOrCreate(path)
	if err != nil {
		t.Fatalf("second LoadOrCreate: %v", err)
	}
	if created {
		t.Fatal("second load should not recreate the file")
	}
}

func TestLoadAppliesDefaultsToOmittedFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	body := `radios:
  - name: One
    url: https://example.com/stream
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("LoadOrCreate: %v", err)
	}
	if cfg.Volume != DefaultVolume {
		t.Errorf("volume = %d, want %d", cfg.Volume, DefaultVolume)
	}
	if cfg.Poll() != DefaultPollInterval {
		t.Errorf("poll = %s, want %s", cfg.Poll(), DefaultPollInterval)
	}
	if !cfg.Autofill {
		t.Error("autofill should default to true")
	}
}

func TestExplicitFalseAutofillSurvives(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	body := `autofill: false
radios:
  - name: One
    url: https://example.com/stream
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := LoadOrCreate(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Autofill {
		t.Fatal("explicit autofill: false was overwritten by defaults")
	}
}

func TestDurationParsing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	body := `poll_interval: 1500ms
radios:
  - name: One
    url: https://example.com/stream
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := LoadOrCreate(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Poll(); got != 1500*time.Millisecond {
		t.Fatalf("poll = %s, want 1.5s", got)
	}
}

func TestValidation(t *testing.T) {
	base := Default()
	tests := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{"empty radios", func(c *Config) { c.Radios = nil }, "at least one"},
		{"volume high", func(c *Config) { c.Volume = 101 }, "volume"},
		{"volume low", func(c *Config) { c.Volume = -1 }, "volume"},
		{"poll too fast", func(c *Config) { c.PollInterval = Duration(10 * time.Millisecond) }, "poll_interval"},
		{"poll too slow", func(c *Config) { c.PollInterval = Duration(time.Minute) }, "poll_interval"},
		{"empty name", func(c *Config) { c.Radios[0].Name = "  " }, "name is empty"},
		{"empty url", func(c *Config) { c.Radios[0].URL = "" }, "url is empty"},
		{"bad scheme", func(c *Config) { c.Radios[0].URL = "file:///tmp/x" }, "http(s)"},
		{"no host", func(c *Config) { c.Radios[0].URL = "https:///x" }, "no host"},
		{"duplicate", func(c *Config) { c.Radios[1].URL = c.Radios[0].URL }, "duplicates"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := base
			cfg.Radios = append([]Radio(nil), base.Radios...)
			tt.mutate(&cfg)
			err := cfg.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() = %v, want error containing %q", err, tt.want)
			}
		})
	}
}

func TestPathPrecedence(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LOFI_CONFIG", "~/custom.yaml")
	if got := Path(); !strings.HasSuffix(got, "custom.yaml") || strings.HasPrefix(got, "~") {
		t.Fatalf("LOFI_CONFIG path = %q", got)
	}

	t.Setenv("LOFI_CONFIG", "")
	t.Setenv("XDG_CONFIG_HOME", dir)
	want := filepath.Join(dir, appDir, "config.yaml")
	if got := Path(); got != want {
		t.Fatalf("XDG path = %q, want %q", got, want)
	}

	t.Setenv("XDG_CONFIG_HOME", "")
	got := Path()
	if !strings.HasSuffix(got, filepath.Join(".config", appDir, "config.yaml")) {
		t.Fatalf("home path = %q", got)
	}
}
