package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/w0rxbend/nerd-font-installer/internal/fontname"
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
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
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
		if err := fontname.Validate(family); err != nil {
			return err
		}
		if seen[family] {
			return fmt.Errorf("duplicate font family %q", family)
		}
		seen[family] = true
	}
	return nil
}

// Discover locates and loads the first existing configuration from the default candidate paths.
// It returns the loaded Source, true and a nil error when a configuration is found;
// returns false and a nil error when no configuration is discovered;
// and returns a non-nil error if computing default paths or loading a discovered configuration fails.
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
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("locate executable: %w", err)
	}
	executableDir := filepath.Dir(executable)

	paths := []string{}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".nerd-config.yaml"))
	}
	if userConfigDir, err := os.UserConfigDir(); err == nil {
		paths = append(paths, filepath.Join(userConfigDir, "nerd-config-installer", "config.yaml"))
	}
	paths = append(paths,
		filepath.Join(executableDir, "config.yaml"),
		filepath.Join(executableDir, "nerd-config.yaml"),
	)
	return paths, nil
}
