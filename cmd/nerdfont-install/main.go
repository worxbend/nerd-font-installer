package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
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
	configPath := flag.String("config", "", "YAML config file; when omitted, discover config or start interactive mode")
	dryRun := flag.Bool("dry-run", false, "print planned downloads without installing fonts")
	showVersion := flag.Bool("version", false, "print version information and exit")
	flag.Parse()

	if *showVersion {
		fmt.Fprintf(os.Stdout, "nerdfont-install %s (%s, %s)\n", version, commit, date)
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	cfg, err := resolveConfig(ctx, *configPath, configFlagWasProvided())
	if err != nil {
		if errors.Is(err, errCancelled) {
			return
		}
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	if err := install(ctx, cfg, *dryRun); err != nil {
		fmt.Fprintf(os.Stderr, "install fonts: %v\n", err)
		os.Exit(1)
	}
}

func resolveConfig(ctx context.Context, configPath string, explicitConfig bool) (config.Config, error) {
	if explicitConfig {
		cfg, err := config.Load(configPath)
		if err != nil {
			return config.Config{}, fmt.Errorf("load config %s: %w", configPath, err)
		}
		return cfg, nil
	}

	source, found, err := config.Discover()
	if err != nil {
		return config.Config{}, err
	}
	if found {
		fmt.Fprintf(os.Stderr, "Using config %s\n", source.Path)
		return source.Config, nil
	}

	if !isTerminal(os.Stdin) || !isTerminal(os.Stdout) {
		return config.Config{}, fmt.Errorf(
			"no config found; pass --config or create ~/.nerd-config.yaml, ~/.config/nerd-config-installer/config.yaml, or config.yaml next to the binary",
		)
	}

	fmt.Fprintln(os.Stderr, "No config found. Starting interactive mode...")
	releases, err := nerdfonts.Client{}.Releases(ctx)
	if err != nil {
		return config.Config{}, err
	}

	result, err := tui.Run(ctx, releases, tui.Options{
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

func configFlagWasProvided() bool {
	provided := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "config" {
			provided = true
		}
	})
	return provided
}

func install(ctx context.Context, cfg config.Config, dryRun bool) error {
	return fonts.Install(ctx, fonts.Options{
		Release:          cfg.Release,
		Destination:      cfg.Destination,
		Families:         cfg.Families,
		RefreshFontCache: cfg.RefreshFontCache,
		DryRun:           dryRun,
		Stdout:           os.Stdout,
		Stderr:           os.Stderr,
	})
}

func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
