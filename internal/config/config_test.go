package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAppliesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fonts.yaml")
	if err := os.WriteFile(path, []byte("families: [JetBrainsMono]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Release != "latest" {
		t.Fatalf("Release = %q", cfg.Release)
	}
	if cfg.Destination != "~/.local/share/fonts/NerdFonts" {
		t.Fatalf("Destination = %q", cfg.Destination)
	}
}

func TestLoadNormalizesValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fonts.yaml")
	data := []byte("release: ' v3.4.0 '\ndestination: ' /tmp/fonts '\nfamilies: [' Hack ', ' JetBrainsMono ']\n")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Release != "v3.4.0" {
		t.Fatalf("Release = %q, want %q", cfg.Release, "v3.4.0")
	}
	if cfg.Destination != "/tmp/fonts" {
		t.Fatalf("Destination = %q, want %q", cfg.Destination, "/tmp/fonts")
	}
	if got := cfg.Families; len(got) != 2 || got[0] != "Hack" || got[1] != "JetBrainsMono" {
		t.Fatalf("Families = %#v", got)
	}
}

func TestLoadRejectsBlankAfterTrim(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{
			name: "release",
			data: "release: '   '\ndestination: /tmp/fonts\nfamilies: [Hack]\n",
		},
		{
			name: "destination",
			data: "release: latest\ndestination: '   '\nfamilies: [Hack]\n",
		},
		{
			name: "family",
			data: "release: latest\ndestination: /tmp/fonts\nfamilies: ['   ']\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "fonts.yaml")
			if err := os.WriteFile(path, []byte(tt.data), 0o644); err != nil {
				t.Fatal(err)
			}

			if _, err := Load(path); err == nil {
				t.Fatal("Load() error = nil, want blank value error")
			}
		})
	}
}

func TestValidateRejectsDuplicateFamilies(t *testing.T) {
	cfg := Config{Release: "latest", Destination: "/tmp/fonts", Families: []string{"Hack", "Hack"}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want duplicate error")
	}
}

func TestValidateRejectsDuplicateFamiliesAfterTrim(t *testing.T) {
	cfg := Config{Release: "latest", Destination: "/tmp/fonts", Families: []string{"Hack", " Hack "}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want duplicate error")
	}
}

func TestValidateRejectsUnsafeFamilyNames(t *testing.T) {
	tests := []struct {
		name   string
		family string
	}{
		{name: "slash", family: "Hack/Regular"},
		{name: "backslash", family: `Hack\Regular`},
		{name: "absolute", family: "/tmp/Hack"},
		{name: "dot", family: "."},
		{name: "dot dot", family: ".."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{Release: "latest", Destination: "/tmp/fonts", Families: []string{tt.family}}
			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want unsafe family error")
			}
		})
	}
}

func TestDiscoverPathsUsesFirstExistingConfig(t *testing.T) {
	temp := t.TempDir()
	missing := filepath.Join(temp, "missing.yaml")
	first := filepath.Join(temp, "first.yaml")
	second := filepath.Join(temp, "second.yaml")

	if err := os.WriteFile(first, []byte("families: [Hack]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte("families: [JetBrainsMono]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	source, found, err := DiscoverPaths([]string{missing, first, second})
	if err != nil {
		t.Fatalf("DiscoverPaths() error = %v", err)
	}
	if !found {
		t.Fatal("DiscoverPaths() found = false")
	}
	if source.Path != first {
		t.Fatalf("Path = %q, want %q", source.Path, first)
	}
	if got := source.Config.Families; len(got) != 1 || got[0] != "Hack" {
		t.Fatalf("Families = %#v", got)
	}
}

func TestDiscoverPathsReturnsInvalidConfigError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(path, []byte("families: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, err := DiscoverPaths([]string{path})
	if err == nil {
		t.Fatal("DiscoverPaths() error = nil, want validation error")
	}
}
