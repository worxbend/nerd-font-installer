package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/w0rxbend/nerd-font-installer/internal/config"
	"github.com/w0rxbend/nerd-font-installer/internal/fonts"
	"github.com/w0rxbend/nerd-font-installer/internal/nerdfonts"
	"github.com/w0rxbend/nerd-font-installer/internal/tui"
)

func TestRunPrintsVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run(t.Context(), []string{"--version"}, strings.NewReader(""), &stdout, &stderr, dependencies{})
	if code != 0 {
		t.Fatalf("run() code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "nerdfont-install") {
		t.Fatalf("stdout = %q, want version", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunPrintsFontNamesForLatestRelease(t *testing.T) {
	t.Setenv(configEnvVar, "") // keep hermetic against a stray ambient override
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	installCalled := false

	code := run(t.Context(), []string{"--font-names"}, strings.NewReader(""), &stdout, &stderr, dependencies{
		discoverConfig: func() (config.Source, bool, error) {
			return config.Source{}, false, nil
		},
		listReleases: func(context.Context) ([]nerdfonts.Release, error) {
			return []nerdfonts.Release{
				{TagName: "v3.4.0", Families: []string{"Hack", "JetBrainsMono"}},
				{TagName: "v3.3.0", Families: []string{"FiraCode"}},
			}, nil
		},
		installFonts: func(context.Context, fonts.Options) error {
			installCalled = true
			return nil
		},
	})
	if code != 0 {
		t.Fatalf("run() code = %d, want 0; stderr = %q", code, stderr.String())
	}
	want := "# v3.4.0\nfamilies:\n  - Hack\n  - JetBrainsMono\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
	if installCalled {
		t.Fatal("install should not be called")
	}
}

func TestRunPrintsFontNamesForConfiguredRelease(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run(t.Context(), []string{"--font-names", "--config", "fonts.yaml"}, strings.NewReader(""), &stdout, &stderr, dependencies{
		loadConfig: func(path string) (config.Config, error) {
			if path != "fonts.yaml" {
				t.Fatalf("load path = %q, want fonts.yaml", path)
			}
			return config.Config{Release: "v3.3.0", Destination: "/tmp/fonts", Families: []string{"Hack"}}, nil
		},
		listReleases: func(context.Context) ([]nerdfonts.Release, error) {
			return []nerdfonts.Release{
				{TagName: "v3.4.0", Families: []string{"Hack"}},
				{TagName: "v3.3.0", Families: []string{"FiraCode", "Meslo"}},
			}, nil
		},
	})
	if code != 0 {
		t.Fatalf("run() code = %d, want 0; stderr = %q", code, stderr.String())
	}
	want := "# v3.3.0\nfamilies:\n  - FiraCode\n  - Meslo\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
}

func TestRunPrintsFontNamesReportsMissingRelease(t *testing.T) {
	var stderr bytes.Buffer

	code := run(t.Context(), []string{"--font-names", "--config", "fonts.yaml"}, strings.NewReader(""), &bytes.Buffer{}, &stderr, dependencies{
		loadConfig: func(string) (config.Config, error) {
			return config.Config{Release: "v1.0.0", Destination: "/tmp/fonts", Families: []string{"Hack"}}, nil
		},
		listReleases: func(context.Context) ([]nerdfonts.Release, error) {
			return []nerdfonts.Release{{TagName: "v3.4.0", Families: []string{"Hack"}}}, nil
		},
	})
	if code != 2 {
		t.Fatalf("run() code = %d, want 2 (user-correctable: unknown release)", code)
	}
	if !strings.Contains(stderr.String(), `release "v1.0.0" was not found`) {
		t.Fatalf("stderr = %q, want missing release", stderr.String())
	}
}

func TestRunReturnsUsageCodeForInvalidFlags(t *testing.T) {
	var stderr bytes.Buffer

	code := run(t.Context(), []string{"--bogus"}, strings.NewReader(""), &bytes.Buffer{}, &stderr, dependencies{})
	if code != 2 {
		t.Fatalf("run() code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "flag provided but not defined") {
		t.Fatalf("stderr = %q, want flag error", stderr.String())
	}
}

func TestRunReturnsUsageCodeForInvalidIconMode(t *testing.T) {
	var stderr bytes.Buffer

	code := run(t.Context(), []string{"--icons", "sparkles"}, strings.NewReader(""), &bytes.Buffer{}, &stderr, dependencies{})
	if code != 2 {
		t.Fatalf("run() code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "invalid --icons") {
		t.Fatalf("stderr = %q, want icon mode error", stderr.String())
	}
}

func TestRunErrorsWhenNoConfigAndNonInteractive(t *testing.T) {
	var stderr bytes.Buffer

	code := run(t.Context(), nil, strings.NewReader(""), &bytes.Buffer{}, &stderr, dependencies{
		discoverConfig: func() (config.Source, bool, error) {
			return config.Source{}, false, nil
		},
		isTerminal: func(stdin io.Reader, stdout io.Writer) bool {
			return false
		},
	})
	if code != 2 {
		t.Fatalf("run() code = %d, want 2 (user-correctable: missing config)", code)
	}
	if !strings.Contains(stderr.String(), "no config found") {
		t.Fatalf("stderr = %q, want no config error", stderr.String())
	}
}

func TestRunLoadsConfigFromEnvOverride(t *testing.T) {
	t.Setenv(configEnvVar, "/env/fonts.yaml")
	var stderr bytes.Buffer
	var gotPath string
	installed := false

	code := run(t.Context(), nil, strings.NewReader(""), &bytes.Buffer{}, &stderr, dependencies{
		loadConfig: func(path string) (config.Config, error) {
			gotPath = path
			return config.Config{Release: "v3.4.0", Destination: "/tmp/fonts", Families: []string{"Hack"}}, nil
		},
		discoverConfig: func() (config.Source, bool, error) {
			t.Fatal("discovery must not run when the env override is set")
			return config.Source{}, false, nil
		},
		installFonts: func(context.Context, fonts.Options) error {
			installed = true
			return nil
		},
		isTerminal: func(io.Reader, io.Writer) bool { return false },
	})
	if code != 0 {
		t.Fatalf("run() code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if gotPath != "/env/fonts.yaml" {
		t.Fatalf("loaded path = %q, want the env override", gotPath)
	}
	if !installed {
		t.Fatal("install should have run with the env config")
	}
}

func TestRunFontNamesHonorsDiscoveredConfig(t *testing.T) {
	t.Setenv(configEnvVar, "")
	var stdout bytes.Buffer

	code := run(t.Context(), []string{"--font-names"}, strings.NewReader(""), &stdout, &bytes.Buffer{}, dependencies{
		discoverConfig: func() (config.Source, bool, error) {
			return config.Source{Path: "/found.yaml", Config: config.Config{Release: "v3.3.0", Destination: "/tmp", Families: []string{"Hack"}}}, true, nil
		},
		listReleases: func(context.Context) ([]nerdfonts.Release, error) {
			return []nerdfonts.Release{
				{TagName: "v3.4.0", Families: []string{"Hack"}},
				{TagName: "v3.3.0", Families: []string{"FiraCode"}},
			}, nil
		},
	})
	if code != 0 {
		t.Fatalf("run() code = %d, want 0", code)
	}
	if !strings.HasPrefix(stdout.String(), "# v3.3.0\n") {
		t.Fatalf("stdout = %q, want discovered release v3.3.0", stdout.String())
	}
}

func TestRunLoadsExplicitConfigAndInstalls(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	var got fonts.Options

	code := run(t.Context(), []string{"--config", "fonts.yaml", "--dry-run"}, strings.NewReader(""), &stdout, &stderr, dependencies{
		loadConfig: func(path string) (config.Config, error) {
			if path != "fonts.yaml" {
				t.Fatalf("load path = %q, want fonts.yaml", path)
			}
			return config.Config{
				Release:          "v3.4.0",
				Destination:      "/tmp/fonts",
				Families:         []string{"Hack"},
				RefreshFontCache: true,
			}, nil
		},
		installFonts: func(ctx context.Context, opts fonts.Options) error {
			got = opts
			return nil
		},
	})
	if code != 0 {
		t.Fatalf("run() code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if got.Release != "v3.4.0" || got.Destination != "/tmp/fonts" || !got.DryRun || len(got.Families) != 1 || got.Families[0] != "Hack" {
		t.Fatalf("install options = %#v", got)
	}
	if got.Stdout != &stdout || got.Stderr != &stderr {
		t.Fatal("install writers were not passed through")
	}
}

func TestRunReportsExplicitConfigLoadError(t *testing.T) {
	var stderr bytes.Buffer

	code := run(t.Context(), []string{"--config", "missing.yaml"}, strings.NewReader(""), &bytes.Buffer{}, &stderr, dependencies{
		loadConfig: func(string) (config.Config, error) {
			return config.Config{}, errors.New("missing")
		},
	})
	if code != 1 {
		t.Fatalf("run() code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "load config missing.yaml") {
		t.Fatalf("stderr = %q, want load context", stderr.String())
	}
}

func TestRunUsesDiscoveredConfig(t *testing.T) {
	var stderr bytes.Buffer
	installed := false

	code := run(t.Context(), nil, strings.NewReader(""), &bytes.Buffer{}, &stderr, dependencies{
		discoverConfig: func() (config.Source, bool, error) {
			return config.Source{
				Path: "discovered.yaml",
				Config: config.Config{
					Release:     "latest",
					Destination: "/tmp/fonts",
					Families:    []string{"Hack"},
				},
			}, true, nil
		},
		installFonts: func(context.Context, fonts.Options) error {
			installed = true
			return nil
		},
	})
	if code != 0 {
		t.Fatalf("run() code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if !installed {
		t.Fatal("install was not called")
	}
	if !strings.Contains(stderr.String(), "Using config discovered.yaml") {
		t.Fatalf("stderr = %q, want discovery message", stderr.String())
	}
}

func TestRunInteractiveCancellationIsSuccess(t *testing.T) {
	var stderr bytes.Buffer
	var gotIcons tui.IconMode

	code := run(t.Context(), []string{"--icons", "nerd"}, strings.NewReader(""), &bytes.Buffer{}, &stderr, dependencies{
		discoverConfig: func() (config.Source, bool, error) {
			return config.Source{}, false, nil
		},
		isTerminal: func(stdin io.Reader, stdout io.Writer) bool {
			return true
		},
		listReleases: func(context.Context) ([]nerdfonts.Release, error) {
			return []nerdfonts.Release{{Name: "v3.4.0", TagName: "v3.4.0", Families: []string{"Hack"}}}, nil
		},
		runTUI: func(_ context.Context, _ []nerdfonts.Release, opts tui.Options) (tui.Result, error) {
			gotIcons = opts.Icons
			return tui.Result{Cancelled: true}, nil
		},
	})
	if code != 0 {
		t.Fatalf("run() code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if gotIcons != tui.IconNerd {
		t.Fatalf("TUI icons = %q, want %q", gotIcons, tui.IconNerd)
	}
}
