# Nerd Font Installer

`nerdfont-install` is a small, script-friendly Go CLI for installing selected
[Nerd Fonts](https://github.com/ryanoasis/nerd-fonts) from release archives.

It is built for repeatable workstation setup: keep the font list in YAML, pin a
release when you need reproducibility, preview the work with `--dry-run`, and
install each family into a predictable directory.

## Why This Exists

Manual font installation is easy once and annoying forever. Bootstrap scripts,
dotfiles, and fresh machines need something more boring:

- a declarative list of font families
- a clear destination directory
- optional release pinning
- an interactive picker when no config exists
- no prompts when config is present
- useful dry-run output
- optional `fc-cache` refresh on Linux

That is the whole job of this tool.

## Features

- Downloads Nerd Font `.zip` archives from GitHub releases.
- Supports `latest` or a pinned release tag such as `v3.4.0`.
- Discovers config from home, XDG config, or the binary directory.
- Starts a Charm Bracelet TUI when no config is available.
- Lists Nerd Fonts releases and selectable font archive names in interactive mode.
- Extracts only font files: `.ttf`, `.otf`, and `.ttc`.
- Installs each family into its own directory.
- Expands `~` in destination paths.
- Skips `fc-cache` cleanly when it is not installed.
- Writes diagnostics to stderr and normal progress to stdout.
- Accepts `Ctrl+C` through context cancellation.

## Installation

Build from source:

```bash
go build -trimpath -o bin/nerdfont-install ./cmd/nerdfont-install
```

Check the binary:

```bash
./bin/nerdfont-install --version
```

The project currently targets Go 1.26.

## Quick Start

Create a config file:

```bash
mkdir -p ~/.config/nerd-config-installer
cp config.example.yaml ~/.config/nerd-config-installer/config.yaml
```

Preview the install:

```bash
./bin/nerdfont-install --dry-run
```

Install the fonts:

```bash
./bin/nerdfont-install
```

Or run without any config and use the interactive TUI:

```bash
./bin/nerdfont-install
```

The TUI fetches Nerd Fonts releases, lets you select a release, then lets you
select one or more font families to install.

## Configuration

Configuration is YAML:

```yaml
release: latest
destination: ~/.local/share/fonts/NerdFonts
refresh_font_cache: true
families:
  - JetBrainsMono
  - Hack
  - FiraCode
  - Meslo
```

### Fields

| Field | Required | Default | Description |
| --- | --- | --- | --- |
| `release` | No | `latest` | Nerd Fonts release source. Use `latest` or a tag such as `v3.4.0`. |
| `destination` | No | `~/.local/share/fonts/NerdFonts` | Root directory where font family directories are created. |
| `refresh_font_cache` | No | `false` | Runs `fc-cache -f <destination>` after installation when enabled and available. |
| `families` | Yes | none | Nerd Font archive names to install. Names must match release asset names. |

### Discovery Order

When `--config` is not provided, the CLI looks for config in this order:

1. `~/.nerd-config.yaml`
2. `~/.config/nerd-config-installer/config.yaml`
3. `config.yaml` next to the runnable binary
4. `nerd-config.yaml` next to the runnable binary

If none of those files exists and the process is attached to a terminal,
interactive mode starts automatically. In non-interactive environments, pass
`--config` explicitly.

Family names map directly to Nerd Fonts release assets. For example,
`JetBrainsMono` with `latest` downloads:

```text
https://github.com/ryanoasis/nerd-fonts/releases/latest/download/JetBrainsMono.zip
```

With a pinned release:

```yaml
release: v3.4.0
families:
  - JetBrainsMono
```

the download URL becomes:

```text
https://github.com/ryanoasis/nerd-fonts/releases/download/v3.4.0/JetBrainsMono.zip
```

## CLI Reference

```text
nerdfont-install [flags]
```

| Flag | Default | Description |
| --- | --- | --- |
| `--config` | none | YAML config file with release, destination, cache refresh, and families. When omitted, discovery runs first, then interactive mode. |
| `--dry-run` | `false` | Print planned downloads and destinations without writing files. |
| `--font-names` | `false` | Print YAML-ready font family names for the latest release, or for the release from `--config`, then exit. |
| `--version` | `false` | Print build metadata and exit. |

## Install Layout

Given this config:

```yaml
destination: ~/.local/share/fonts/NerdFonts
families:
  - JetBrainsMono
  - Hack
```

the installer writes font files under:

```text
~/.local/share/fonts/NerdFonts/
  JetBrainsMono/
    JetBrainsMonoNerdFont-Regular.ttf
    ...
  Hack/
    HackNerdFont-Regular.ttf
    ...
```

Existing files with the same names are overwritten.

## Usage Examples

### Install One Font Family

Create a small config:

```yaml
release: latest
destination: ~/.local/share/fonts/NerdFonts
refresh_font_cache: true
families:
  - JetBrainsMono
```

Preview it:

```bash
./bin/nerdfont-install --config fonts.yaml --dry-run
```

Install it:

```bash
./bin/nerdfont-install --config fonts.yaml
```

### Install Several Families

Use this when setting up a terminal, editor, and fallback monospace stack:

```yaml
release: latest
destination: ~/.local/share/fonts/NerdFonts
refresh_font_cache: true
families:
  - JetBrainsMono
  - Hack
  - FiraCode
  - Meslo
```

Run:

```bash
./bin/nerdfont-install --config fonts.yaml
```

### Use Discovered Config

Put your standard font config in the XDG config path:

```bash
mkdir -p ~/.config/nerd-config-installer
cp config.example.yaml ~/.config/nerd-config-installer/config.yaml
```

Then run without flags:

```bash
./bin/nerdfont-install --dry-run
./bin/nerdfont-install
```

### Use Interactive Mode

If no discovered config exists, run the binary in a terminal:

```bash
./bin/nerdfont-install
```

Interactive controls:

| Key | Action |
| --- | --- |
| `up` / `down` | Move through releases or font families. |
| `/` | Filter the current list. |
| `enter` | Select a release, or install selected families. |
| `space` | Toggle a font family. |
| `a` | Select or clear all families for the chosen release. |
| `b` / `esc` | Go back from family selection to release selection. |
| `q` / `ctrl+c` | Quit. |

### List Font Names for Config

Print copy-paste-ready family names for the latest Nerd Fonts release:

```bash
./bin/nerdfont-install --font-names
```

Output:

```yaml
# v3.4.0
families:
  - Hack
  - JetBrainsMono
  - Meslo
```

For a pinned release, put the release in a config file and pass it with
`--font-names`:

```bash
./bin/nerdfont-install --config fonts.yaml --font-names
```

### Pin a Nerd Fonts Release

Use a release tag when your dotfiles or machine bootstrap should produce the
same result over time:

```yaml
release: v3.4.0
destination: ~/.local/share/fonts/NerdFonts
refresh_font_cache: true
families:
  - JetBrainsMono
  - Hack
  - FiraCode
```

Run the same command as usual:

```bash
./bin/nerdfont-install --config fonts.yaml
```

### Install Without Refreshing Font Cache

Disable cache refresh when installing on a system without `fc-cache`, in a
container, or in a test directory:

```yaml
release: latest
destination: ~/.local/share/fonts/NerdFonts
refresh_font_cache: false
families:
  - Hack
```

Run:

```bash
./bin/nerdfont-install --config fonts.yaml
```

### Test Installation in a Local Directory

Use a relative destination when you want to inspect extracted files without
touching your user font directory:

```yaml
release: latest
destination: ./tmp/fonts
refresh_font_cache: false
families:
  - Meslo
```

Preview and install:

```bash
./bin/nerdfont-install --config fonts.yaml --dry-run
./bin/nerdfont-install --config fonts.yaml
```

The output goes under:

```text
./tmp/fonts/Meslo/
```

### Use Config Next to the Binary

Place `config.yaml` beside the compiled binary:

```text
bin/
  nerdfont-install
  config.yaml
```

Then run:

```bash
./bin/nerdfont-install --dry-run
./bin/nerdfont-install
```

### Add It to a Bootstrap Script

Build the binary, copy an example config, and run a dry-run before installing:

```bash
set -euo pipefail

go build -trimpath -o bin/nerdfont-install ./cmd/nerdfont-install
mkdir -p ~/.config/nerd-config-installer
cp config.example.yaml ~/.config/nerd-config-installer/config.yaml

./bin/nerdfont-install --dry-run
./bin/nerdfont-install
```

## Operational Notes

- Network access to `github.com` is required.
- Interactive mode uses the GitHub releases API and may be rate limited by GitHub.
- `families` cannot be empty and cannot contain duplicates.
- Empty family names are rejected.
- `fc-cache` is optional. If it is missing, cache refresh is skipped.
- The installer writes only inside the configured destination.
- Downloads use a 10 minute HTTP client timeout.
- The temporary archive is removed after each family is processed.

## Troubleshooting

### `no config found`

Create a discovered config file or pass an explicit path:

```bash
mkdir -p ~/.config/nerd-config-installer
cp config.example.yaml ~/.config/nerd-config-installer/config.yaml
./bin/nerdfont-install --config /path/to/fonts.yaml
```

When running in a terminal, no config starts the interactive TUI instead.

### `duplicate font family "JetBrainsMono"`

Each family should appear once in `families`.

### `download ... 404 Not Found`

The family name or release tag probably does not match a Nerd Fonts release
asset. Check the release page and use the archive name without `.zip`.

### Font does not show up after install

Enable cache refresh:

```yaml
refresh_font_cache: true
```

Or run it manually:

```bash
fc-cache -f ~/.local/share/fonts/NerdFonts
```

Some applications also need to be restarted before newly installed fonts appear.

## Development

Run the test suite:

```bash
go test ./...
```

Run a local build and smoke test:

```bash
go build -trimpath -o bin/nerdfont-install ./cmd/nerdfont-install
./bin/nerdfont-install --config config.example.yaml --dry-run
```

Keep dependencies tidy:

```bash
go mod tidy
```

## Design Notes

The project intentionally stays small:

- `cmd/nerdfont-install` owns flag parsing and process exit behavior.
- `internal/config` owns YAML loading, defaults, and validation.
- `internal/fonts` owns download, extraction, destination layout, and cache refresh.
- `internal/nerdfonts` owns Nerd Fonts GitHub release discovery.
- `internal/tui` owns the Charm Bracelet interactive picker.

That separation keeps the command easy to script while leaving the core install
logic testable without shelling out to the compiled binary.
