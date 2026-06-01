package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Release          string   `yaml:"release"`
	Destination      string   `yaml:"destination"`
	RefreshFontCache bool     `yaml:"refresh_font_cache"`
	Families         []string `yaml:"families"`
}

type Source struct {
	Path   string
	Config Config
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}
	cfg.ApplyDefaults()
	cfg.Normalize()
	return cfg, cfg.Validate()
}

func (c *Config) ApplyDefaults() {
	if c.Release == "" {
		c.Release = "latest"
	}
	if c.Destination == "" {
		c.Destination = "~/.local/share/fonts/NerdFonts"
	}
}

func (c *Config) Normalize() {
	c.Release = strings.TrimSpace(c.Release)
	c.Destination = strings.TrimSpace(c.Destination)
	for i, family := range c.Families {
		c.Families[i] = strings.TrimSpace(family)
	}
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Release) == "" {
		return fmt.Errorf("release is required")
	}
	if strings.TrimSpace(c.Destination) == "" {
		return fmt.Errorf("destination is required")
	}
	if len(c.Families) == 0 {
		return fmt.Errorf("at least one font family is required")
	}
	seen := map[string]bool{}
	for _, family := range c.Families {
		family = strings.TrimSpace(family)
		if err := validateFamilyName(family); err != nil {
			return err
		}
		if seen[family] {
			return fmt.Errorf("duplicate font family %q", family)
		}
		seen[family] = true
	}
	return nil
}

func validateFamilyName(family string) error {
	switch {
	case family == "":
		return fmt.Errorf("font family names cannot be empty")
	case family == "." || family == "..":
		return fmt.Errorf("unsafe font family name %q", family)
	case strings.Contains(family, "/") || strings.Contains(family, "\\"):
		return fmt.Errorf("unsafe font family name %q", family)
	case filepath.IsAbs(family):
		return fmt.Errorf("unsafe font family name %q", family)
	case filepath.Base(family) != family:
		return fmt.Errorf("unsafe font family name %q", family)
	default:
		return nil
	}
}

func Discover() (Source, bool, error) {
	paths, err := DefaultPaths()
	if err != nil {
		return Source{}, false, err
	}
	return DiscoverPaths(paths)
}

func DiscoverPaths(paths []string) (Source, bool, error) {
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			continue
		}
		cfg, err := Load(path)
		if err == nil {
			return Source{Path: path, Config: cfg}, true, nil
		}
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		return Source{}, false, fmt.Errorf("load discovered config %s: %w", path, err)
	}
	return Source{}, false, nil
}

func DefaultPaths() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("locate home directory: %w", err)
	}
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("locate executable: %w", err)
	}
	executableDir := filepath.Dir(executable)

	return []string{
		filepath.Join(home, ".nerd-config.yaml"),
		filepath.Join(home, ".config", "nerd-config-installer", "config.yaml"),
		filepath.Join(executableDir, "config.yaml"),
		filepath.Join(executableDir, "nerd-config.yaml"),
	}, nil
}
