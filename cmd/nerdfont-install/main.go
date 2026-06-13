package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"github.com/w0rxbend/nerd-font-installer/internal/config"
	"github.com/w0rxbend/nerd-font-installer/internal/fonts"
	"github.com/w0rxbend/nerd-font-installer/internal/nerdfonts"
	"github.com/w0rxbend/nerd-font-installer/internal/tui"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"

	errCancelled = errors.New("cancelled")
	errNoConfig  = errors.New("no config found")
)

// configEnvVar names an environment variable holding a config path. It is
// honored like --config (highest priority after the explicit flag), which suits
// dotfiles, CI, and containers.
const configEnvVar = "NERDFONT_CONFIG"

// effectiveConfigPath resolves which config path to load and whether it is an
// explicit selection: the --config flag wins, then $NERDFONT_CONFIG, otherwise
// effectiveConfigPath chooses which configuration path to use and reports whether
// it was explicitly supplied.
//
// It prefers an explicitly provided path (explicit == true). If not explicit, it
// uses the path from the NERDFONT_CONFIG environment variable when set (and
// marks that as explicit). If neither applies, it returns the original
// configPath and marks it as not explicit.
func effectiveConfigPath(configPath string, explicit bool) (string, bool) {
	if explicit {
		return configPath, true
	}
	if env := strings.TrimSpace(os.Getenv(configEnvVar)); env != "" {
		return env, true
	}
	return configPath, false
}

// main is the program entry point. It creates a context canceled on SIGINT, invokes run with command-line
// arguments and the standard streams, and exits the process with the returned status code.
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	os.Exit(run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr, dependencies{}))
}

type dependencies struct {
	loadConfig     func(string) (config.Config, error)
	discoverConfig func() (config.Source, bool, error)
	listReleases   func(context.Context) ([]nerdfonts.Release, error)
	runTUI         func(context.Context, []nerdfonts.Release, tui.Options) (tui.Result, error)
	installFonts   func(context.Context, fonts.Options) error
	isTerminal     func(io.Reader, io.Writer) bool
}

func (d dependencies) withDefaults() dependencies {
	if d.loadConfig == nil {
		d.loadConfig = config.Load
	}
	if d.discoverConfig == nil {
		d.discoverConfig = config.Discover
	}
	if d.listReleases == nil {
		d.listReleases = nerdfonts.Client{}.Releases
	}
	if d.runTUI == nil {
		d.runTUI = tui.Run
	}
	if d.installFonts == nil {
		d.installFonts = fonts.Install
	}
	if d.isTerminal == nil {
		d.isTerminal = isTerminal
	}
	return d
}

// run parses CLI arguments, resolves or discovers the configuration (falling back to interactive mode when appropriate), and orchestrates font installation, returning an exit code for the process.
// It applies default dependency implementations, handles flags (--config, --dry-run, --font-names, --icons, --version), writes diagnostics to stderr/stdout, and maps known error conditions to specific exit codes.
func run(
	ctx context.Context,
	args []string,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
	deps dependencies,
) int {
	deps = deps.withDefaults()

	flags := flag.NewFlagSet("nerdfont-install", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "", "YAML config file; when omitted, discover config or start interactive mode")
	dryRun := flags.Bool("dry-run", false, "print planned downloads without installing fonts")
	showFontNames := flags.Bool("font-names", false, "print YAML-ready Nerd Font family names and exit")
	iconMode := flags.String("icons", string(tui.IconAuto), "TUI icon mode: auto, nerd, unicode, or ascii")
	showVersion := flags.Bool("version", false, "print version information and exit")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	icons, err := parseIconMode(*iconMode)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}

	if *showVersion {
		_, _ = fmt.Fprintf(stdout, "nerdfont-install %s (%s, %s)\n", version, commit, date)
		return 0
	}

	explicitConfig := false
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "config" {
			explicitConfig = true
		}
	})

	if *showFontNames {
		if printErr := printFontNames(ctx, *configPath, explicitConfig, stdout, deps); printErr != nil {
			_, _ = fmt.Fprintf(stderr, "%v\n", printErr)
			return exitCodeFor(printErr)
		}
		return 0
	}

	cfg, err := resolveConfig(
		ctx,
		*configPath,
		explicitConfig,
		deps.isTerminal(stdin, stdout),
		icons,
		stderr,
		deps,
	)
	if err != nil {
		if errors.Is(err, errCancelled) {
			return 0
		}
		_, _ = fmt.Fprintf(stderr, "%v\n", err)
		return exitCodeFor(err)
	}

	if err := install(ctx, cfg, *dryRun, stdout, stderr, deps.installFonts); err != nil {
		_, _ = fmt.Fprintf(stderr, "install fonts: %v\n", err)
		return 1
	}
	return 0
}

// exitCodeFor maps an error to a process exit code: 2 for user-input problems
// the caller can correct (missing config, unknown or absent release), 1 for
// exitCodeFor maps an error to the process exit code.
// It returns 2 if the error indicates a missing configuration, no available releases,
// or a specific release was not found; it returns 1 for all other errors.
func exitCodeFor(err error) int {
	var notFound nerdfonts.ReleaseNotFoundError
	switch {
	case errors.As(err, &notFound),
		errors.Is(err, nerdfonts.ErrNoReleases),
		errors.Is(err, errNoConfig):
		return 2
	default:
		return 1
	}
}

// printFontNames writes a YAML-style list of font family names for the resolved release to stdout.
// It resolves the release by using an explicit config path (including NERDFONT_CONFIG) when present,
// otherwise it attempts to discover a config; if neither provides a release it uses the latest release.
// It fetches available releases, selects the requested release, and prints a header `# <TagName>`
// followed by `families:` and each family as `  - <family>`.
// Returns an error if config loading, discovery, release listing, or release selection fails.
func printFontNames(
	ctx context.Context,
	configPath string,
	explicitConfig bool,
	stdout io.Writer,
	deps dependencies,
) error {
	release := nerdfonts.Latest
	if path, explicit := effectiveConfigPath(configPath, explicitConfig); explicit {
		cfg, err := deps.loadConfig(path)
		if err != nil {
			return fmt.Errorf("load config %s: %w", path, err)
		}
		release = cfg.Release
	} else if source, found, err := deps.discoverConfig(); err != nil {
		return err
	} else if found {
		release = source.Config.Release
	}

	releases, err := deps.listReleases(ctx)
	if err != nil {
		return err
	}
	selected, err := selectRelease(releases, release)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintf(stdout, "# %s\nfamilies:\n", selected.TagName)
	for _, family := range selected.Families {
		_, _ = fmt.Fprintf(stdout, "  - %s\n", family)
	}
	return nil
}

// noConfigError builds the "no config found" message from the live discovery
// noConfigError returns an error indicating that no configuration was found and suggests how to provide one.
// The error wraps errNoConfig and includes a hint that recommends passing --config, setting NERDFONT_CONFIG,
// or creating a config at one of the platform-specific default paths when available.
func noConfigError() error {
	hint := fmt.Sprintf("pass --config or set %s", configEnvVar)
	if paths, err := config.DefaultPaths(); err == nil && len(paths) > 0 {
		hint = fmt.Sprintf("pass --config, set %s, or create one of: %s", configEnvVar, strings.Join(paths, ", "))
	}
	return fmt.Errorf("%w; %s", errNoConfig, hint)
}

// selectRelease selects a release from the provided list that matches the requested tag.
// If `release` is empty or equals `nerdfonts.Latest`, the first release in the slice is returned.
// If the slice is empty, `nerdfonts.ErrNoReleases` is returned.
// If no release with a matching TagName is found, `nerdfonts.ReleaseNotFoundError{Tag: release}` is returned.
func selectRelease(releases []nerdfonts.Release, release string) (nerdfonts.Release, error) {
	if len(releases) == 0 {
		return nerdfonts.Release{}, nerdfonts.ErrNoReleases
	}
	if release == "" || release == nerdfonts.Latest {
		return releases[0], nil
	}
	for _, candidate := range releases {
		if candidate.TagName == release {
			return candidate, nil
		}
	}
	return nerdfonts.Release{}, nerdfonts.ReleaseNotFoundError{Tag: release}
}

// mode runs, the function returns errCancelled if the user cancels the TUI.
func resolveConfig(
	ctx context.Context,
	configPath string,
	explicitConfig bool,
	terminal bool,
	icons tui.IconMode,
	stderr io.Writer,
	deps dependencies,
) (config.Config, error) {
	if path, explicit := effectiveConfigPath(configPath, explicitConfig); explicit {
		cfg, err := deps.loadConfig(path)
		if err != nil {
			return config.Config{}, fmt.Errorf("load config %s: %w", path, err)
		}
		return cfg, nil
	}

	source, found, err := deps.discoverConfig()
	if err != nil {
		return config.Config{}, err
	}
	if found {
		_, _ = fmt.Fprintf(stderr, "Using config %s\n", source.Path)
		return source.Config, nil
	}

	if !terminal {
		return config.Config{}, noConfigError()
	}

	_, _ = fmt.Fprintln(stderr, "No config found. Starting interactive mode...")
	releases, err := tui.LoadReleases(ctx, deps.listReleases, stderr)
	if err != nil {
		return config.Config{}, err
	}

	result, err := deps.runTUI(ctx, releases, tui.Options{
		Destination:      "~/.local/share/fonts/NerdFonts",
		RefreshFontCache: true,
		Icons:            icons,
	})
	if err != nil {
		return config.Config{}, err
	}
	if result.Cancelled {
		return config.Config{}, errCancelled
	}
	return result.Config, nil
}

func parseIconMode(raw string) (tui.IconMode, error) {
	mode := tui.IconMode(strings.ToLower(strings.TrimSpace(raw)))
	switch mode {
	case tui.IconAuto, tui.IconNerd, tui.IconUnicode, tui.IconASCII:
		return mode, nil
	default:
		return "", fmt.Errorf("invalid --icons %q; use auto, nerd, unicode, or ascii", raw)
	}
}

func install(
	ctx context.Context,
	cfg config.Config,
	dryRun bool,
	stdout io.Writer,
	stderr io.Writer,
	installFonts func(context.Context, fonts.Options) error,
) error {
	return installFonts(ctx, fonts.Options{
		Release:          cfg.Release,
		Destination:      cfg.Destination,
		Families:         cfg.Families,
		RefreshFontCache: cfg.RefreshFontCache,
		DryRun:           dryRun,
		Stdout:           stdout,
		Stderr:           stderr,
	})
}

func isTerminal(stdin io.Reader, stdout io.Writer) bool {
	stdinFile, stdinOK := stdin.(*os.File)
	stdoutFile, stdoutOK := stdout.(*os.File)
	if !stdinOK || !stdoutOK {
		return false
	}

	stdinInfo, err := stdinFile.Stat()
	if err != nil {
		return false
	}
	stdoutInfo, err := stdoutFile.Stat()
	if err != nil {
		return false
	}
	return stdinInfo.Mode()&os.ModeCharDevice != 0 && stdoutInfo.Mode()&os.ModeCharDevice != 0
}
