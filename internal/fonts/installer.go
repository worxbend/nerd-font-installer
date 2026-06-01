package fonts

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Options struct {
	Release          string
	Destination      string
	Families         []string
	RefreshFontCache bool
	DryRun           bool
	Stdout           io.Writer
	Stderr           io.Writer
	HTTPClient       *http.Client
}

func Install(ctx context.Context, opts Options) error {
	if opts.Stdout == nil {
		opts.Stdout = io.Discard
	}
	if opts.Stderr == nil {
		opts.Stderr = io.Discard
	}
	if opts.HTTPClient == nil {
		opts.HTTPClient = &http.Client{Timeout: 10 * time.Minute}
	}
	opts = normalizeOptions(opts)
	if opts.Release == "" {
		opts.Release = "latest"
	}
	if err := validateOptions(opts); err != nil {
		return err
	}

	root, err := expandPath(opts.Destination)
	if err != nil {
		return err
	}

	if opts.DryRun {
		for _, family := range opts.Families {
			fmt.Fprintf(opts.Stdout, "Would install %s from %s into %s\n", family, ReleaseURL(opts.Release, family), filepath.Join(root, family))
		}
		if opts.RefreshFontCache {
			fmt.Fprintf(opts.Stdout, "Would refresh font cache for %s\n", root)
		}
		return nil
	}

	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("create destination %s: %w", root, err)
	}
	for _, family := range opts.Families {
		if err := installFamily(ctx, opts.HTTPClient, opts.Release, family, root, opts.Stderr); err != nil {
			return fmt.Errorf("install Nerd Font family %s: %w", family, err)
		}
	}

	if opts.RefreshFontCache {
		return refreshFontCache(ctx, root, opts.Stdout, opts.Stderr)
	}
	return nil
}

func normalizeOptions(opts Options) Options {
	opts.Release = strings.TrimSpace(opts.Release)
	opts.Destination = strings.TrimSpace(opts.Destination)
	opts.Families = append([]string(nil), opts.Families...)
	for i, family := range opts.Families {
		opts.Families[i] = strings.TrimSpace(family)
	}
	return opts
}

func validateOptions(opts Options) error {
	if len(opts.Families) == 0 {
		return fmt.Errorf("at least one Nerd Font family is required")
	}
	for _, family := range opts.Families {
		if err := validateFamilyName(family); err != nil {
			return err
		}
	}
	return nil
}

func validateFamilyName(family string) error {
	switch {
	case family == "":
		return fmt.Errorf("font family names cannot be empty")
	case family == "." || family == "..":
		return fmt.Errorf("unsafe Nerd Font family name %q", family)
	case strings.Contains(family, "/") || strings.Contains(family, "\\"):
		return fmt.Errorf("unsafe Nerd Font family name %q", family)
	case filepath.IsAbs(family):
		return fmt.Errorf("unsafe Nerd Font family name %q", family)
	case filepath.Base(family) != family:
		return fmt.Errorf("unsafe Nerd Font family name %q", family)
	default:
		return nil
	}
}

func installFamily(ctx context.Context, client *http.Client, release, family, root string, stderr io.Writer) error {
	url := ReleaseURL(release, family)
	fmt.Fprintf(stderr, "Installing Nerd Font %s from %s\n", family, url)

	temp, err := os.CreateTemp("", "nerd-font-*.zip")
	if err != nil {
		return fmt.Errorf("create temporary zip file: %w", err)
	}
	defer os.Remove(temp.Name())
	defer temp.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create download request %s: %w", url, err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("download %s: %s", url, resp.Status)
	}
	if _, err := io.Copy(temp, resp.Body); err != nil {
		return fmt.Errorf("copy download %s to %s: %w", url, temp.Name(), err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("finalize download %s: %w", temp.Name(), err)
	}

	destination := filepath.Join(root, family)
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return fmt.Errorf("create family destination %s: %w", destination, err)
	}
	if err := ExtractFontZip(temp.Name(), destination); err != nil {
		return fmt.Errorf("extract %s to %s: %w", temp.Name(), destination, err)
	}
	return nil
}

func ReleaseURL(release, family string) string {
	family = url.PathEscape(family)
	if release == "latest" {
		return fmt.Sprintf("https://github.com/ryanoasis/nerd-fonts/releases/latest/download/%s.zip", family)
	}
	return fmt.Sprintf("https://github.com/ryanoasis/nerd-fonts/releases/download/%s/%s.zip", url.PathEscape(release), family)
}

func ExtractFontZip(path, destination string) error {
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return fmt.Errorf("create extraction destination %s: %w", destination, err)
	}
	archive, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("open font zip %s: %w", path, err)
	}
	defer archive.Close()

	extracted := 0
	for _, file := range archive.File {
		if file.FileInfo().IsDir() || !isFontFile(file.Name) {
			continue
		}
		if err := extractZipFile(file, filepath.Join(destination, filepath.Base(file.Name))); err != nil {
			return fmt.Errorf("extract %s: %w", file.Name, err)
		}
		extracted++
	}
	if extracted == 0 {
		return fmt.Errorf("extract %s: no font files found", path)
	}
	return nil
}

func isFontFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".otf", ".ttc", ".ttf":
		return true
	default:
		return false
	}
}

func extractZipFile(file *zip.File, destination string) error {
	reader, err := file.Open()
	if err != nil {
		return fmt.Errorf("open zipped font %s: %w", file.Name, err)
	}
	defer reader.Close()

	out, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("create font file %s: %w", destination, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, reader); err != nil {
		return fmt.Errorf("copy font file %s to %s: %w", file.Name, destination, err)
	}
	return nil
}

func refreshFontCache(ctx context.Context, root string, stdout, stderr io.Writer) error {
	if _, err := exec.LookPath("fc-cache"); err != nil {
		fmt.Fprintln(stderr, "fc-cache is not available; skipping font cache refresh.")
		return nil
	}
	fmt.Fprintln(stderr, "Refreshing font cache...")
	cmd := exec.CommandContext(ctx, "fc-cache", "-f", root)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run fc-cache for %s: %w", root, err)
	}
	return nil
}

func expandPath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("destination is required")
	}
	if path == "~" {
		return os.UserHomeDir()
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, strings.TrimPrefix(path, "~/")), nil
	}
	return path, nil
}
