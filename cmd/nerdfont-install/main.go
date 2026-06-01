package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"

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
)

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
	showVersion := flags.Bool("version", false, "print version information and exit")
	if err := flags.Parse(args); err != nil {
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
		if err := printFontNames(ctx, *configPath, explicitConfig, stdout, deps); err != nil {
			_, _ = fmt.Fprintf(stderr, "%v\n", err)
			return 1
		}
		return 0
	}

	cfg, err := resolveConfig(
		ctx,
		*configPath,
		explicitConfig,
		deps.isTerminal(stdin, stdout),
		stderr,
		deps,
	)
	if err != nil {
		if errors.Is(err, errCancelled) {
			return 0
		}
		_, _ = fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}

	if err := install(ctx, cfg, *dryRun, stdout, stderr, deps.installFonts); err != nil {
		_, _ = fmt.Fprintf(stderr, "install fonts: %v\n", err)
		return 1
	}
	return 0
}

func printFontNames(
	ctx context.Context,
	configPath string,
	explicitConfig bool,
	stdout io.Writer,
	deps dependencies,
) error {
	release := "latest"
	if explicitConfig {
		cfg, err := deps.loadConfig(configPath)
		if err != nil {
			return fmt.Errorf("load config %s: %w", configPath, err)
		}
		release = cfg.Release
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

func selectRelease(releases []nerdfonts.Release, release string) (nerdfonts.Release, error) {
	if len(releases) == 0 {
		return nerdfonts.Release{}, fmt.Errorf("no Nerd Fonts releases found")
	}
	if release == "" || release == "latest" {
		return releases[0], nil
	}
	for _, candidate := range releases {
		if candidate.TagName == release {
			return candidate, nil
		}
	}
	return nerdfonts.Release{}, fmt.Errorf("nerd fonts release %q was not found", release)
}

func resolveConfig(
	ctx context.Context,
	configPath string,
	explicitConfig bool,
	terminal bool,
	stderr io.Writer,
	deps dependencies,
) (config.Config, error) {
	if explicitConfig {
		cfg, err := deps.loadConfig(configPath)
		if err != nil {
			return config.Config{}, fmt.Errorf("load config %s: %w", configPath, err)
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
		return config.Config{}, fmt.Errorf(
			"no config found; pass --config or create ~/.nerd-config.yaml, ~/.config/nerd-config-installer/config.yaml, or config.yaml next to the binary",
		)
	}

	_, _ = fmt.Fprintln(stderr, "No config found. Starting interactive mode...")
	releases, err := tui.LoadReleases(ctx, deps.listReleases, stderr)
	if err != nil {
		return config.Config{}, err
	}

	result, err := deps.runTUI(ctx, releases, tui.Options{
		Destination:      "~/.local/share/fonts/NerdFonts",
		RefreshFontCache: true,
	})
	if err != nil {
		return config.Config{}, err
	}
	if result.Cancelled {
		return config.Config{}, errCancelled
	}
	return result.Config, nil
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
