package fonts

import (
	"archive/zip"
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/w0rxbend/nerd-font-installer/internal/fontname"
	"github.com/w0rxbend/nerd-font-installer/internal/nerdfonts"
	"golang.org/x/sync/errgroup"
)

const (
	// maxConcurrentInstalls bounds how many font families download and extract
	// at once. Downloads are latency/bandwidth bound and all target one host, so
	// a small fan-out captures most of the speedup without stressing the server.
	maxConcurrentInstalls = 4
	// perDownloadTimeout bounds a single family's download+extract, derived per
	// family so it composes with group cancellation instead of a global clock.
	perDownloadTimeout = 10 * time.Minute
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

// Size caps that bound resource use against an oversized or malicious archive
// (e.g. a decompression bomb). They are package variables rather than constants
// so tests can lower them without building multi-megabyte fixtures.
var (
	maxDownloadBytes int64 = 200 << 20 // 200 MiB: cap on a single downloaded zip.
	maxFontFileBytes int64 = 64 << 20  // 64 MiB: cap on one extracted font file.
	maxArchiveBytes  int64 = 512 << 20 // 512 MiB: cap on total uncompressed bytes.
)

var (
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	warnStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	spinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))
	fontStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("81")).Bold(true)
	linkStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Underline(true)
	pathStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("219"))
)

func Install(ctx context.Context, opts Options) error {
	if opts.Stdout == nil {
		opts.Stdout = io.Discard
	}
	if opts.Stderr == nil {
		opts.Stderr = io.Discard
	}
	if opts.HTTPClient == nil {
		// Per-download timeouts are applied via context, so leave the client
		// timeout unset. Raise the per-host connection caps from the default 2
		// so concurrent downloads to github.com reuse pooled connections.
		if defaultTransport, ok := http.DefaultTransport.(*http.Transport); ok {
			transport := defaultTransport.Clone()
			transport.MaxIdleConnsPerHost = maxConcurrentInstalls
			transport.MaxConnsPerHost = maxConcurrentInstalls
			opts.HTTPClient = &http.Client{Transport: transport}
		} else {
			opts.HTTPClient = &http.Client{Transport: http.DefaultTransport}
		}
	}
	opts = normalizeOptions(opts)
	if opts.Release == "" {
		opts.Release = nerdfonts.Latest
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
			_, _ = fmt.Fprintf(
				opts.Stdout,
				"%s Would install %s from %s into %s\n",
				spinnerStyle.Render("•"),
				fontStyle.Render(family),
				linkStyle.Render(ReleaseURL(opts.Release, family)),
				pathStyle.Render(filepath.Join(root, family)),
			)
		}
		if opts.RefreshFontCache {
			_, _ = fmt.Fprintf(opts.Stdout, "%s Would refresh font cache for %s\n", spinnerStyle.Render("↻"), pathStyle.Render(root))
		}
		return nil
	}

	if err := os.MkdirAll(root, 0o755); err != nil { //nolint:gosec // Font directories must be traversable by font tooling and user applications.
		return fmt.Errorf("create destination %s: %w", root, err)
	}

	// Each family writes to disjoint filesystem paths (root/<family>, a unique
	// MkdirTemp staging dir, and a per-family ".old" backup), so installs are
	// independent and safe to run concurrently. Dedupe first as a defensive
	// guard: two workers on the same family name would collide on those paths.
	families := dedupeFamilies(opts.Families)
	stderr := &syncWriter{w: opts.Stderr}

	// Fetch the release's published SHA-256 manifest once. Verification is
	// best-effort: a missing manifest leaves checksums nil and installs proceed
	// unverified (with a warning); a hash mismatch later is fatal.
	checksums := fetchChecksums(ctx, opts.HTTPClient, opts.Release, stderr)

	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(min(len(families), maxConcurrentInstalls))
	for _, family := range families {
		group.Go(func() error {
			if err := installFamily(groupCtx, opts.HTTPClient, opts.Release, family, root, checksums[family], stderr); err != nil {
				return fmt.Errorf("install Nerd Font family %s: %w", family, err)
			}
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return err
	}

	if opts.RefreshFontCache {
		return refreshFontCache(ctx, root, opts.Stdout, opts.Stderr)
	}
	return nil
}

// syncWriter serializes whole-line progress writes from concurrent installs so
// lines from different families never interleave mid-line. lipgloss rendering
// happens in memory before Write, so only the Write needs guarding.
type syncWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (s *syncWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Write(p)
}

func dedupeFamilies(families []string) []string {
	seen := make(map[string]bool, len(families))
	unique := make([]string, 0, len(families))
	for _, family := range families {
		if seen[family] {
			continue
		}
		seen[family] = true
		unique = append(unique, family)
	}
	return unique
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
		if err := fontname.Validate(family); err != nil {
			return err
		}
	}
	return nil
}

func installFamily(ctx context.Context, client *http.Client, release, family, root, wantChecksum string, stderr io.Writer) error {
	ctx, cancel := context.WithTimeout(ctx, perDownloadTimeout)
	defer cancel()

	url := ReleaseURL(release, family)
	_, _ = fmt.Fprintf(stderr, "%s Installing Nerd Font %s from %s\n", spinnerStyle.Render("⠋"), fontStyle.Render(family), linkStyle.Render(url))

	temp, err := os.CreateTemp("", "nerd-font-*.zip")
	if err != nil {
		return fmt.Errorf("create temporary zip file: %w", err)
	}
	defer func() {
		_ = os.Remove(temp.Name())
	}()
	// Safety net for the early-return error paths below; the meaningful close
	// that surfaces flush errors happens explicitly after io.Copy. Re-closing
	// an already-closed file returns os.ErrClosed, which is harmless here.
	defer func() {
		_ = temp.Close()
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create download request %s: %w", url, err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("download %s: %s", url, resp.Status)
	}
	if resp.ContentLength > maxDownloadBytes {
		return fmt.Errorf("download %s: size %d bytes exceeds %d byte limit", url, resp.ContentLength, maxDownloadBytes)
	}
	// LimitReader backstops a missing or dishonest Content-Length; the extra
	// byte lets us detect a stream that runs past the cap. Tee through a hasher
	// so the download can be verified against the published checksum.
	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(temp, hasher), io.LimitReader(resp.Body, maxDownloadBytes+1))
	if err != nil {
		return fmt.Errorf("copy download %s to %s: %w", url, temp.Name(), err)
	}
	if written > maxDownloadBytes {
		return fmt.Errorf("download %s: exceeds %d byte limit", url, maxDownloadBytes)
	}
	if closeErr := temp.Close(); closeErr != nil {
		return fmt.Errorf("finalize download %s: %w", temp.Name(), closeErr)
	}
	if wantChecksum != "" {
		if got := hex.EncodeToString(hasher.Sum(nil)); got != wantChecksum {
			return fmt.Errorf("checksum mismatch for %s: downloaded sha256 %s, expected %s", family, got, wantChecksum)
		}
	}

	destination := filepath.Join(root, family)
	tempDestination, err := os.MkdirTemp(root, "."+family+"-*")
	if err != nil {
		return fmt.Errorf("create temporary family destination in %s: %w", root, err)
	}
	defer func() {
		_ = os.RemoveAll(tempDestination)
	}()

	if err := ExtractFontZip(temp.Name(), tempDestination); err != nil {
		return fmt.Errorf("extract %s to %s: %w", temp.Name(), tempDestination, err)
	}
	if err := replaceDirectory(tempDestination, destination); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(stderr, "%s Installed %s into %s\n", successStyle.Render("✅"), fontStyle.Render(family), pathStyle.Render(destination))
	return nil
}

// ChecksumURL returns the URL of the SHA-256 manifest published alongside a
// release's font archives.
func ChecksumURL(release string) string {
	if release == "" || release == nerdfonts.Latest {
		return "https://github.com/ryanoasis/nerd-fonts/releases/latest/download/SHA-256.txt"
	}
	return fmt.Sprintf("https://github.com/ryanoasis/nerd-fonts/releases/download/%s/SHA-256.txt", url.PathEscape(release))
}

// fetchChecksums downloads and parses the release's SHA-256 manifest into a map
// of family name to lowercase hex digest. Verification is best-effort: if the
// manifest cannot be fetched it warns and returns nil so installs proceed
// unverified. Only a later digest mismatch (in installFamily) is fatal.
func fetchChecksums(ctx context.Context, client *http.Client, release string, stderr io.Writer) map[string]string {
	checksumURL := ChecksumURL(release)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checksumURL, nil)
	if err != nil {
		warnChecksums(stderr, err)
		return nil
	}
	resp, err := client.Do(req)
	if err != nil {
		warnChecksums(stderr, err)
		return nil
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		warnChecksums(stderr, fmt.Errorf("%s", resp.Status))
		return nil
	}

	checksums := map[string]string{}
	scanner := bufio.NewScanner(io.LimitReader(resp.Body, 1<<20))
	for scanner.Scan() {
		// Each line is "<sha256 hex>  <filename>"; keep only the .zip archives.
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 || !strings.EqualFold(filepath.Ext(fields[1]), ".zip") {
			continue
		}
		family := strings.TrimSuffix(fields[1], filepath.Ext(fields[1]))
		checksums[family] = strings.ToLower(fields[0])
	}
	if err := scanner.Err(); err != nil {
		// A partially-read manifest is treated as unavailable rather than fatal;
		// any family it did not cover simply installs unverified.
		warnChecksums(stderr, err)
	}
	return checksums
}

func warnChecksums(stderr io.Writer, cause error) {
	_, _ = fmt.Fprintf(stderr, "%s Checksum manifest unavailable (%v); installing without integrity verification.\n", warnStyle.Render("•"), cause)
}

func ReleaseURL(release, family string) string {
	family = url.PathEscape(family)
	if release == "" || release == nerdfonts.Latest {
		return fmt.Sprintf("https://github.com/ryanoasis/nerd-fonts/releases/latest/download/%s.zip", family)
	}
	return fmt.Sprintf("https://github.com/ryanoasis/nerd-fonts/releases/download/%s/%s.zip", url.PathEscape(release), family)
}

func ExtractFontZip(path, destination string) error {
	if err := os.MkdirAll(destination, 0o755); err != nil { //nolint:gosec // Extracted font directories need normal user/app traversal permissions.
		return fmt.Errorf("create extraction destination %s: %w", destination, err)
	}
	archive, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("open font zip %s: %w", path, err)
	}
	defer func() {
		_ = archive.Close()
	}()

	extracted := 0
	var totalBytes int64
	for _, file := range archive.File {
		if file.FileInfo().IsDir() || !isFontFile(file.Name) {
			continue
		}
		// Reject on the declared size first (cheap, no decompression), then
		// enforce the same cap on the actual stream during the copy.
		if exceedsInt64Limit(file.UncompressedSize64, maxFontFileBytes) {
			return fmt.Errorf("extract %s: font file %s declares %d bytes, exceeds %d byte limit", path, file.Name, file.UncompressedSize64, maxFontFileBytes)
		}
		entryBytes := int64(file.UncompressedSize64) //nolint:gosec // exceedsInt64Limit above rejects values that cannot fit in int64.
		if totalBytes > maxArchiveBytes-entryBytes {
			return fmt.Errorf("extract %s: total uncompressed size exceeds %d byte limit", path, maxArchiveBytes)
		}
		totalBytes += entryBytes
		if totalBytes > maxArchiveBytes {
			return fmt.Errorf("extract %s: total uncompressed size exceeds %d byte limit", path, maxArchiveBytes)
		}
		if err := extractZipFile(file, filepath.Join(destination, filepath.Base(file.Name)), maxFontFileBytes); err != nil {
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

func exceedsInt64Limit(value uint64, limit int64) bool {
	if limit < 0 || value > math.MaxInt64 {
		return true
	}
	return int64(value) > limit
}

func extractZipFile(file *zip.File, destination string, limit int64) error {
	reader, err := file.Open()
	if err != nil {
		return fmt.Errorf("open zipped font %s: %w", file.Name, err)
	}
	defer func() {
		_ = reader.Close()
	}()

	out, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644) //nolint:gosec // Installed fonts should be readable by normal font consumers.
	if err != nil {
		return fmt.Errorf("create font file %s: %w", destination, err)
	}
	// Safety net only; the meaningful close is the explicit one below, where a
	// flush error (ENOSPC, EIO, quota) on a written file would otherwise be
	// silently dropped and a truncated font promoted as a successful install.
	defer func() {
		_ = out.Close()
	}()

	// Backstop the declared-size check against a zip entry whose header lies
	// about UncompressedSize64; the extra byte detects an over-limit stream.
	written, err := io.Copy(out, io.LimitReader(reader, limit+1))
	if err != nil {
		return fmt.Errorf("copy font file %s to %s: %w", file.Name, destination, err)
	}
	if written > limit {
		return fmt.Errorf("font file %s exceeds %d byte limit", file.Name, limit)
	}
	if err := out.Sync(); err != nil {
		return fmt.Errorf("flush font file %s: %w", destination, err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("finalize font file %s: %w", destination, err)
	}
	return nil
}

func replaceDirectory(source, destination string) error {
	backup := destination + ".old"
	if err := os.RemoveAll(backup); err != nil {
		return fmt.Errorf("remove old backup %s: %w", backup, err)
	}

	destinationExists := true
	if _, err := os.Stat(destination); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("inspect existing destination %s: %w", destination, err)
		}
		destinationExists = false
	}

	if destinationExists {
		if err := os.Rename(destination, backup); err != nil {
			return fmt.Errorf("move existing destination %s to %s: %w", destination, backup, err)
		}
	}

	if err := os.Rename(source, destination); err != nil {
		if destinationExists {
			_ = os.Rename(backup, destination)
		}
		return fmt.Errorf("move extracted fonts %s to %s: %w", source, destination, err)
	}

	// The swap above is the commit point: the new fonts are now live at
	// destination. Removing the backup is best-effort cleanup and must not turn
	// a succeeded install into a reported failure. A leftover ".old" directory
	// is harmless and is cleared by the RemoveAll at the top of the next run.
	_ = os.RemoveAll(backup)
	return nil
}

func refreshFontCache(ctx context.Context, root string, stdout, stderr io.Writer) error {
	if _, err := exec.LookPath("fc-cache"); err != nil {
		if !errors.Is(err, exec.ErrNotFound) {
			return fmt.Errorf("look up fc-cache: %w", err)
		}
		_, _ = fmt.Fprintf(stderr, "%s fc-cache is not available; skipping font cache refresh.\n", warnStyle.Render("•"))
		return nil
	}
	_, _ = fmt.Fprintf(stderr, "%s Refreshing font cache for %s\n", spinnerStyle.Render("⠋"), pathStyle.Render(root))
	cmd := exec.CommandContext(ctx, "fc-cache", "-f", root)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run fc-cache for %s: %w", root, err)
	}
	_, _ = fmt.Fprintf(stderr, "%s Font cache refreshed\n", successStyle.Render("✅"))
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
