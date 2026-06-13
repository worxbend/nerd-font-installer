# Nerd Font Installer

Install your favorite [Nerd Fonts](https://github.com/ryanoasis/nerd-fonts)
with one small command.

`nerdfont-install` is for people who set up terminals, editors, dotfiles, new
laptops, remote dev boxes, or fresh Linux/macOS machines and do not want to
manually download font archives every time.

Instead of clicking through GitHub releases, unzipping files, moving fonts into
the right folder, and refreshing the font cache by hand, you keep a short YAML
file:

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

Then run:

```bash
nerdfont-install
```

That is it.

## Why You Might Want This

Nerd Fonts are great. Installing them repeatedly is not.

This tool is useful when you:

- want terminal icons and glyphs to work in Starship, Neovim, tmux, lazygit,
  eza, yazi, WezTerm, Alacritty, Kitty, Ghostty, or VS Code
- rebuild machines often
- maintain dotfiles
- bootstrap dev environments
- want the same fonts on every workstation
- want a repeatable setup script instead of manual clicking
- want to preview exactly what will be installed before writing files

Manual install:

1. Open the Nerd Fonts release page.
2. Find the right font archive.
3. Download it.
4. Unzip it.
5. Move only the font files.
6. Put them in a font directory.
7. Refresh the font cache.
8. Repeat for every font.

With `nerdfont-install`:

```bash
nerdfont-install --dry-run
nerdfont-install
```

## What It Does

- Downloads Nerd Font release archives from GitHub.
- Installs only font files: `.ttf`, `.otf`, and `.ttc`.
- Keeps each font family in its own folder.
- Supports `latest` or pinned releases like `v3.4.0`.
- Reads a simple YAML config.
- Can discover your config automatically.
- Has an interactive picker when you do not have a config yet.
- Prints copy-paste-ready font family names for configs.
- Supports dry-runs.
- Refreshes `fc-cache` on Linux when requested.
- Skips `fc-cache` safely if it is not installed.
- Uses colorful CLI output and a Charm Bubble Tea TUI.

## Quick Start for Beginners

### 1. Get the Binary

Download the latest release archive for your system from the GitHub releases
page.

Pick one:

| System | Archive |
| --- | --- |
| Linux Intel/AMD | `nerdfont-install_latest_linux_amd64.tar.gz` |
| Linux ARM64 | `nerdfont-install_latest_linux_arm64.tar.gz` |
| macOS Intel | `nerdfont-install_latest_darwin_amd64.tar.gz` |
| macOS Apple Silicon | `nerdfont-install_latest_darwin_arm64.tar.gz` |

Extract it:

```bash
tar -xzf nerdfont-install_latest_linux_amd64.tar.gz
```

Move it somewhere on your `PATH`:

```bash
chmod +x nerdfont-install
mkdir -p ~/.local/bin
mv nerdfont-install ~/.local/bin/
```

Check it works:

```bash
nerdfont-install --version
```

If `~/.local/bin` is not on your `PATH`, add this to your shell config:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

### 2. Create a Config

Create the config directory:

```bash
mkdir -p ~/.config/nerd-config-installer
```

Create the config file:

```bash
nano ~/.config/nerd-config-installer/config.yaml
```

Paste this:

```yaml
release: latest
destination: ~/.local/share/fonts/NerdFonts
refresh_font_cache: true
families:
  - JetBrainsMono
  - Hack
  - FiraCode
```

Save the file.

### 3. Preview First

Before installing, run a dry-run:

```bash
nerdfont-install --dry-run
```

This prints what would be downloaded and where it would go. It does not write
font files.

### 4. Install

Run:

```bash
nerdfont-install
```

Now select the installed Nerd Font in your terminal or editor settings.

## The Easiest Mode: Interactive Picker

If you do not want to write YAML yet, just run:

```bash
nerdfont-install
```

When no config file is found, the app opens an interactive picker:

1. Pick a Nerd Fonts release.
2. Pick one or more font families.
3. Press `enter`.
4. The selected fonts install.

Controls:

| Key | Action |
| --- | --- |
| `up` / `down` | Move through releases or fonts |
| `/` | Search/filter |
| `enter` | Choose a release or confirm selected fonts |
| `space` | Select or unselect a font |
| `a` | Select all or clear all |
| `b` / `esc` | Go back |
| `q` / `ctrl+c` | Quit |

## Copy-Paste Font Names Into Your Config

Not sure what the exact family names are? Ask the tool:

```bash
nerdfont-install --font-names
```

It prints YAML you can paste directly into your config:

```yaml
# v3.4.0
families:
  - 0xProto
  - 3270
  - AdwaitaMono
  - Agave
  - AnonymousPro
```

For a pinned release, put the release in your config and run:

```bash
nerdfont-install --config ~/.config/nerd-config-installer/config.yaml --font-names
```

## Common Recipes

### Install One Font

```yaml
release: latest
destination: ~/.local/share/fonts/NerdFonts
refresh_font_cache: true
families:
  - JetBrainsMono
```

Run:

```bash
nerdfont-install --config fonts.yaml --dry-run
nerdfont-install --config fonts.yaml
```

### Install a Good Terminal Font Set

```yaml
release: latest
destination: ~/.local/share/fonts/NerdFonts
refresh_font_cache: true
families:
  - JetBrainsMono
  - Hack
  - FiraCode
  - Meslo
  - SymbolsOnly
```

### Pin a Release for Dotfiles

Use this when you want the same result every time your setup script runs.

```yaml
release: v3.4.0
destination: ~/.local/share/fonts/NerdFonts
refresh_font_cache: true
families:
  - JetBrainsMono
  - Hack
```

### Install Into a Test Folder

Use this if you want to inspect the files before touching your real font
directory.

```yaml
release: latest
destination: ./tmp/fonts
refresh_font_cache: false
families:
  - Hack
```

Run:

```bash
nerdfont-install --config fonts.yaml
```

Files will appear under:

```text
./tmp/fonts/Hack/
```

### Use It in a Bootstrap Script

```bash
#!/usr/bin/env bash
set -euo pipefail

mkdir -p ~/.config/nerd-config-installer

cat > ~/.config/nerd-config-installer/config.yaml <<'YAML'
release: latest
destination: ~/.local/share/fonts/NerdFonts
refresh_font_cache: true
families:
  - JetBrainsMono
  - Hack
  - FiraCode
YAML

nerdfont-install --dry-run
nerdfont-install
```

This is useful in dotfiles, Ansible roles, install scripts, or fresh-machine
setup scripts.

## Configuration Reference

Config is YAML.

```yaml
release: latest
destination: ~/.local/share/fonts/NerdFonts
refresh_font_cache: true
families:
  - JetBrainsMono
  - Hack
```

| Field | Required | Default | Meaning |
| --- | --- | --- | --- |
| `release` | No | `latest` | Nerd Fonts release to use. Use `latest` or a tag like `v3.4.0`. |
| `destination` | No | `~/.local/share/fonts/NerdFonts` | Root folder where fonts are installed. |
| `refresh_font_cache` | No | `false` | Run `fc-cache -f <destination>` after installing. |
| `families` | Yes | none | Font archive names without `.zip`. |

Family names must be exact Nerd Font archive names. Use `--font-names` when in
doubt.

## Where Config Files Are Found

When `--config` is not provided, the tool checks the `NERDFONT_CONFIG`
environment variable, then the following locations in order:

1. `~/.nerd-config.yaml`
2. `~/.config/nerd-config-installer/config.yaml`
3. `config.yaml` next to the binary
4. `nerd-config.yaml` next to the binary

Set `NERDFONT_CONFIG=/path/to/fonts.yaml` to point at a config without passing
`--config` every time — handy in dotfiles, CI, and containers. It is honored by
`--font-names` as well.

The recommended location is:

```text
~/.config/nerd-config-installer/config.yaml
```

## Command Reference

```text
nerdfont-install [flags]
```

| Flag | What it does |
| --- | --- |
| `--config <path>` | Use a specific YAML config file. |
| `--dry-run` | Show what would happen without installing. |
| `--font-names` | Print YAML-ready font family names and exit. |
| `--icons <mode>` | Set interactive TUI icons: `auto`, `nerd`, `unicode`, or `ascii`. Defaults to `auto`, which avoids requiring Nerd Font glyphs. |
| `--version` | Print version info and exit. |

Examples:

```bash
nerdfont-install --dry-run
nerdfont-install --config fonts.yaml
nerdfont-install --config fonts.yaml --dry-run
nerdfont-install --font-names
nerdfont-install --icons nerd
nerdfont-install --icons ascii
nerdfont-install --version
```

## Install Layout

Given:

```yaml
destination: ~/.local/share/fonts/NerdFonts
families:
  - JetBrainsMono
  - Hack
```

The tool writes:

```text
~/.local/share/fonts/NerdFonts/
  JetBrainsMono/
    JetBrainsMonoNerdFont-Regular.ttf
    ...
  Hack/
    HackNerdFont-Regular.ttf
    ...
```

Each family gets its own directory. Existing files for that family are replaced
after the new archive extracts successfully.

## Troubleshooting

### `no config found`

You have two choices:

1. Create a config:

   ```bash
   mkdir -p ~/.config/nerd-config-installer
   nano ~/.config/nerd-config-installer/config.yaml
   ```

2. Or pass a config explicitly:

   ```bash
   nerdfont-install --config /path/to/fonts.yaml
   ```

If you are in a real terminal and no config exists, interactive mode should
start automatically.

### `duplicate font family "JetBrainsMono"`

The same font appears twice in `families`.

Remove the duplicate:

```yaml
families:
  - JetBrainsMono
```

### `download ... 404 Not Found`

Usually this means the family name or release tag is wrong.

Run:

```bash
nerdfont-install --font-names
```

Then copy the exact family name from the output.

### Font Installed but Not Visible

Try refreshing the font cache:

```bash
fc-cache -f ~/.local/share/fonts/NerdFonts
```

Also restart the application where you select fonts. Terminals and editors often
need a restart before new fonts appear.

### Icons Still Look Broken

Install a Nerd Font and then choose that exact Nerd Font in your terminal or
editor preferences. Installing the font is only half of the job; your app still
needs to use it.

For example, choose something like:

```text
JetBrainsMono Nerd Font
Hack Nerd Font
FiraCode Nerd Font
```

## Notes for Linux and macOS

Linux:

- Recommended destination: `~/.local/share/fonts/NerdFonts`
- Set `refresh_font_cache: true`
- The tool will run `fc-cache` when available

macOS:

- You can use a custom destination if you manage fonts manually
- `fc-cache` is usually not installed and will be skipped
- You may prefer installing into a local folder first, then importing fonts with
  Font Book or another font manager

## Build From Source

Requirements:

- Go 1.26 or newer

Build:

```bash
go build -trimpath -o bin/nerdfont-install ./cmd/nerdfont-install
```

Smoke test:

```bash
./bin/nerdfont-install --version
./bin/nerdfont-install --config config.example.yaml --dry-run
```

## Development

Run the full local validation suite:

```bash
make verify
```

Run tests:

```bash
make test
```

Run vet:

```bash
make vet
```

Run lint:

```bash
make lint
```

Format Go code:

```bash
make fmt
```

## License

MIT. See [LICENSE](LICENSE).

## Design

The project is intentionally small:

- `cmd/nerdfont-install` owns flags, command flow, and exit codes.
- `internal/config` owns YAML loading, defaults, validation, and discovery.
- `internal/fonts` owns downloads, extraction, atomic replacement, and cache refresh.
- `internal/nerdfonts` owns GitHub release discovery.
- `internal/tui` owns the Charm Bubble Tea interactive UI.

The goal is not to be a full font manager. The goal is to make Nerd Font
installation boring, repeatable, and scriptable.
