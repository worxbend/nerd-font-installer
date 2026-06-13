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

// Unit tests for effectiveConfigPath.

func TestEffectiveConfigPathFlagWins(t *testing.T) {
	t.Setenv(configEnvVar, "/env/fonts.yaml")
	got, explicit := effectiveConfigPath("/flag/fonts.yaml", true)
	if got != "/flag/fonts.yaml" || !explicit {
		t.Fatalf("effectiveConfigPath = %q, %v; want /flag/fonts.yaml, true (flag wins over env)", got, explicit)
	}
}

func TestEffectiveConfigPathEnvOverride(t *testing.T) {
	t.Setenv(configEnvVar, "/env/fonts.yaml")
	got, explicit := effectiveConfigPath("", false)
	if got != "/env/fonts.yaml" || !explicit {
		t.Fatalf("effectiveConfigPath = %q, %v; want /env/fonts.yaml, true", got, explicit)
	}
}

func TestEffectiveConfigPathEnvTrimmed(t *testing.T) {
	t.Setenv(configEnvVar, "  /trimmed/fonts.yaml  ")
	got, explicit := effectiveConfigPath("", false)
	if got != "/trimmed/fonts.yaml" || !explicit {
		t.Fatalf("effectiveConfigPath = %q, %v; want trimmed env path, true", got, explicit)
	}
}

func TestEffectiveConfigPathEmptyEnvFallsThrough(t *testing.T) {
	t.Setenv(configEnvVar, "")
	got, explicit := effectiveConfigPath("/default", false)
	if got != "/default" || explicit {
		t.Fatalf("effectiveConfigPath = %q, %v; want /default, false (empty env is no override)", got, explicit)
	}
}

func TestEffectiveConfigPathWhitespaceOnlyEnvFallsThrough(t *testing.T) {
	t.Setenv(configEnvVar, "   ")
	got, explicit := effectiveConfigPath("/fallback", false)
	if got != "/fallback" || explicit {
		t.Fatalf("effectiveConfigPath = %q, %v; want /fallback, false (whitespace-only env is not an override)", got, explicit)
	}
}

// Unit tests for exitCodeFor.

func TestExitCodeForReleaseNotFound(t *testing.T) {
	err := nerdfonts.ReleaseNotFoundError{Tag: "v9.9.9"}
	if got := exitCodeFor(err); got != 2 {
		t.Fatalf("exitCodeFor(ReleaseNotFoundError) = %d, want 2", got)
	}
}

func TestExitCodeForWrappedReleaseNotFound(t *testing.T) {
	wrapped := errors.Join(errors.New("outer"), nerdfonts.ReleaseNotFoundError{Tag: "v9.9.9"})
	if got := exitCodeFor(wrapped); got != 2 {
		t.Fatalf("exitCodeFor(wrapped ReleaseNotFoundError) = %d, want 2", got)
	}
}

func TestExitCodeForNoReleases(t *testing.T) {
	if got := exitCodeFor(nerdfonts.ErrNoReleases); got != 2 {
		t.Fatalf("exitCodeFor(ErrNoReleases) = %d, want 2", got)
	}
}

func TestExitCodeForNoConfig(t *testing.T) {
	// errNoConfig is unexported; verify exitCodeFor handles a wrapped sentinel.
	err := noConfigError()
	if got := exitCodeFor(err); got != 2 {
		t.Fatalf("exitCodeFor(noConfigError()) = %d, want 2", got)
	}
}

func TestExitCodeForRuntimeError(t *testing.T) {
	if got := exitCodeFor(errors.New("network failure")); got != 1 {
		t.Fatalf("exitCodeFor(generic error) = %d, want 1", got)
	}
}

// Unit tests for noConfigError.

func TestNoConfigErrorWrapsErrNoConfig(t *testing.T) {
	err := noConfigError()
	if !errors.Is(err, errNoConfig) {
		t.Fatalf("noConfigError() does not wrap errNoConfig: %v", err)
	}
}

func TestNoConfigErrorMentionsEnvVar(t *testing.T) {
	err := noConfigError()
	if !strings.Contains(err.Error(), configEnvVar) {
		t.Fatalf("noConfigError() = %q, want mention of %s", err.Error(), configEnvVar)
	}
}

func TestNoConfigErrorMentionsPassConfig(t *testing.T) {
	err := noConfigError()
	if !strings.Contains(err.Error(), "--config") {
		t.Fatalf("noConfigError() = %q, want mention of --config flag", err.Error())
	}
}

// Unit tests for selectRelease.

func TestSelectReleaseReturnsFirstForLatest(t *testing.T) {
	releases := []nerdfonts.Release{
		{TagName: "v3.4.0"},
		{TagName: "v3.3.0"},
	}
	got, err := selectRelease(releases, nerdfonts.Latest)
	if err != nil || got.TagName != "v3.4.0" {
		t.Fatalf("selectRelease(%q) = %q, %v; want v3.4.0, nil", nerdfonts.Latest, got.TagName, err)
	}
}

func TestSelectReleaseReturnsFirstForEmptyRelease(t *testing.T) {
	releases := []nerdfonts.Release{{TagName: "v3.4.0"}}
	got, err := selectRelease(releases, "")
	if err != nil || got.TagName != "v3.4.0" {
		t.Fatalf("selectRelease(\"\") = %q, %v; want v3.4.0, nil", got.TagName, err)
	}
}

func TestSelectReleaseFindsNamedTag(t *testing.T) {
	releases := []nerdfonts.Release{
		{TagName: "v3.4.0"},
		{TagName: "v3.3.0"},
	}
	got, err := selectRelease(releases, "v3.3.0")
	if err != nil || got.TagName != "v3.3.0" {
		t.Fatalf("selectRelease(v3.3.0) = %q, %v; want v3.3.0, nil", got.TagName, err)
	}
}

func TestSelectReleaseReturnsNotFoundErrorForMissingTag(t *testing.T) {
	releases := []nerdfonts.Release{{TagName: "v3.4.0"}}
	_, err := selectRelease(releases, "v9.9.9")
	var notFound nerdfonts.ReleaseNotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("selectRelease(missing) error type = %T, want ReleaseNotFoundError; err = %v", err, err)
	}
	if notFound.Tag != "v9.9.9" {
		t.Fatalf("ReleaseNotFoundError.Tag = %q, want v9.9.9", notFound.Tag)
	}
}

func TestSelectReleaseReturnsErrNoReleasesForEmptyList(t *testing.T) {
	_, err := selectRelease(nil, nerdfonts.Latest)
	if !errors.Is(err, nerdfonts.ErrNoReleases) {
		t.Fatalf("selectRelease(nil) error = %v, want ErrNoReleases", err)
	}
}

func TestSelectReleaseReturnsErrNoReleasesForEmptySlice(t *testing.T) {
	_, err := selectRelease([]nerdfonts.Release{}, "v3.4.0")
	if !errors.Is(err, nerdfonts.ErrNoReleases) {
		t.Fatalf("selectRelease(empty slice) error = %v, want ErrNoReleases", err)
	}
}

// Regression: font-names with no explicit config and no discovered config must
// fall back to listReleases for the latest release, not fail.
func TestRunFontNamesWithNoConfigListsLatest(t *testing.T) {
	t.Setenv(configEnvVar, "")
	var stdout bytes.Buffer

	code := run(t.Context(), []string{"--font-names"}, strings.NewReader(""), &stdout, &bytes.Buffer{}, dependencies{
		discoverConfig: func() (config.Source, bool, error) {
			return config.Source{}, false, nil
		},
		listReleases: func(context.Context) ([]nerdfonts.Release, error) {
			return []nerdfonts.Release{
				{TagName: "v3.5.0", Families: []string{"Inter"}},
			}, nil
		},
	})
	if code != 0 {
		t.Fatalf("run() code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "v3.5.0") {
		t.Fatalf("stdout = %q, want latest release v3.5.0", stdout.String())
	}
}

// Regression: the env override must also flow through --font-names, not only
// the install path.
func TestRunFontNamesHonorsEnvOverride(t *testing.T) {
	t.Setenv(configEnvVar, "/env/fonts.yaml")
	var stdout bytes.Buffer

	code := run(t.Context(), []string{"--font-names"}, strings.NewReader(""), &stdout, &bytes.Buffer{}, dependencies{
		loadConfig: func(path string) (config.Config, error) {
			if path != "/env/fonts.yaml" {
				t.Fatalf("loadConfig path = %q, want /env/fonts.yaml", path)
			}
			return config.Config{Release: "v3.2.0", Destination: "/tmp", Families: []string{"Hack"}}, nil
		},
		discoverConfig: func() (config.Source, bool, error) {
			t.Fatal("discoverConfig must not run when env override is set")
			return config.Source{}, false, nil
		},
		listReleases: func(context.Context) ([]nerdfonts.Release, error) {
			return []nerdfonts.Release{
				{TagName: "v3.4.0", Families: []string{"Hack"}},
				{TagName: "v3.2.0", Families: []string{"FiraCode"}},
			}, nil
		},
	})
	if code != 0 {
		t.Fatalf("run() code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "v3.2.0") {
		t.Fatalf("stdout = %q, want env release v3.2.0", stdout.String())
	}
}
