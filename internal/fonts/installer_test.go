package fonts

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseURL(t *testing.T) {
	tests := []struct {
		name    string
		release string
		family  string
		want    string
	}{
		{
			name:    "latest",
			release: "latest",
			family:  "JetBrainsMono",
			want:    "https://github.com/ryanoasis/nerd-fonts/releases/latest/download/JetBrainsMono.zip",
		},
		{
			name:    "tagged release",
			release: "v3.4.0",
			family:  "Hack",
			want:    "https://github.com/ryanoasis/nerd-fonts/releases/download/v3.4.0/Hack.zip",
		},
		{
			name:    "escapes path segments",
			release: "release candidate",
			family:  "Symbols Nerd Font",
			want:    "https://github.com/ryanoasis/nerd-fonts/releases/download/release%20candidate/Symbols%20Nerd%20Font.zip",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ReleaseURL(tt.release, tt.family); got != tt.want {
				t.Fatalf("ReleaseURL(%q, %q) = %q, want %q", tt.release, tt.family, got, tt.want)
			}
		})
	}
}

func TestExtractFontZipOnlyExtractsFonts(t *testing.T) {
	temp := t.TempDir()
	archivePath := filepath.Join(temp, "font.zip")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for name, body := range map[string]string{
		"Font.ttf":        "font",
		"nested/Font.otf": "font",
		"README.md":       "docs",
	} {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	destination := filepath.Join(temp, "out")
	if err := ExtractFontZip(archivePath, destination); err != nil {
		t.Fatalf("ExtractFontZip() error = %v", err)
	}
	for _, name := range []string{"Font.ttf", "Font.otf"} {
		if _, err := os.Stat(filepath.Join(destination, name)); err != nil {
			t.Fatalf("expected extracted font %s: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(destination, "README.md")); !os.IsNotExist(err) {
		t.Fatalf("README.md should not be extracted, stat err = %v", err)
	}
}

func TestExtractFontZipRejectsInvalidZip(t *testing.T) {
	temp := t.TempDir()
	archivePath := filepath.Join(temp, "font.zip")
	if err := os.WriteFile(archivePath, []byte("not a zip"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := ExtractFontZip(archivePath, filepath.Join(temp, "out"))
	if err == nil {
		t.Fatal("ExtractFontZip() error = nil, want invalid zip error")
	}
	if !strings.Contains(err.Error(), "open font zip") {
		t.Fatalf("ExtractFontZip() error = %v, want open font zip context", err)
	}
}

func TestExtractFontZipRejectsArchiveWithoutFonts(t *testing.T) {
	temp := t.TempDir()
	archivePath := filepath.Join(temp, "font.zip")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	entry, err := writer.Create("README.md")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("docs")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	err = ExtractFontZip(archivePath, filepath.Join(temp, "out"))
	if err == nil {
		t.Fatal("ExtractFontZip() error = nil, want empty archive error")
	}
	if !strings.Contains(err.Error(), "no font files found") {
		t.Fatalf("ExtractFontZip() error = %v, want no font files found", err)
	}
}

func TestInstallDryRun(t *testing.T) {
	var stdout bytes.Buffer
	err := Install(t.Context(), Options{
		Release:          "latest",
		Destination:      "/tmp/fonts",
		Families:         []string{"Hack"},
		RefreshFontCache: true,
		DryRun:           true,
		Stdout:           &stdout,
	})
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if !strings.Contains(stdout.String(), "Would install Hack") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "Would refresh font cache") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestInstallDryRunRejectsInvalidFamily(t *testing.T) {
	var stdout bytes.Buffer
	err := Install(t.Context(), Options{
		Release:     "latest",
		Destination: "/tmp/fonts",
		Families:    []string{"../Hack"},
		DryRun:      true,
		Stdout:      &stdout,
	})
	if err == nil {
		t.Fatal("Install() error = nil, want invalid family error")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestInstallProgressWritesToStderr(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	client := &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       io.NopCloser(bytes.NewReader(fontZip(t))),
			}, nil
		}),
	}

	err := Install(t.Context(), Options{
		Release:     "latest",
		Destination: filepath.Join(t.TempDir(), "fonts"),
		Families:    []string{"Hack"},
		Stdout:      &stdout,
		Stderr:      &stderr,
		HTTPClient:  client,
	})
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if strings.Contains(stdout.String(), "Installing Nerd Font") {
		t.Fatalf("stdout = %q, want no progress message", stdout.String())
	}
	if !strings.Contains(stderr.String(), "Installing Nerd Font Hack") {
		t.Fatalf("stderr = %q, want progress message", stderr.String())
	}
}

func TestInstallReplacesFamilyDirectoryAfterSuccessfulExtraction(t *testing.T) {
	var stderr bytes.Buffer
	destination := filepath.Join(t.TempDir(), "fonts")
	existingFamily := filepath.Join(destination, "Hack")
	if err := os.MkdirAll(existingFamily, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(existingFamily, "old.ttf"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	client := &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       io.NopCloser(bytes.NewReader(fontZip(t))),
			}, nil
		}),
	}

	err := Install(t.Context(), Options{
		Release:     "latest",
		Destination: destination,
		Families:    []string{"Hack"},
		Stderr:      &stderr,
		HTTPClient:  client,
	})
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(existingFamily, "Hack.ttf")); err != nil {
		t.Fatalf("expected new font: %v", err)
	}
	if _, err := os.Stat(filepath.Join(existingFamily, "old.ttf")); !os.IsNotExist(err) {
		t.Fatalf("old font should be removed, stat err = %v", err)
	}
}

func TestInstallKeepsExistingFamilyDirectoryOnExtractionFailure(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "fonts")
	existingFamily := filepath.Join(destination, "Hack")
	if err := os.MkdirAll(existingFamily, 0o755); err != nil {
		t.Fatal(err)
	}
	existingFont := filepath.Join(existingFamily, "old.ttf")
	if err := os.WriteFile(existingFont, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	client := &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       io.NopCloser(bytes.NewReader(noFontZip(t))),
			}, nil
		}),
	}

	err := Install(t.Context(), Options{
		Release:     "latest",
		Destination: destination,
		Families:    []string{"Hack"},
		HTTPClient:  client,
	})
	if err == nil {
		t.Fatal("Install() error = nil, want extraction error")
	}
	if data, err := os.ReadFile(existingFont); err != nil || string(data) != "old" {
		t.Fatalf("existing font = %q, %v; want old font preserved", data, err)
	}
}

func TestExtractFontZipRejectsOversizeFontFile(t *testing.T) {
	prev := maxFontFileBytes
	maxFontFileBytes = 8
	t.Cleanup(func() { maxFontFileBytes = prev })

	temp := t.TempDir()
	archivePath := filepath.Join(temp, "font.zip")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	entry, err := writer.Create("Big.ttf")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("this font is larger than the cap")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	err = ExtractFontZip(archivePath, filepath.Join(temp, "out"))
	if err == nil {
		t.Fatal("ExtractFontZip() error = nil, want oversize error")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("ExtractFontZip() error = %v, want size-limit error", err)
	}
}

func TestExtractFontZipRejectsOversizeArchiveTotal(t *testing.T) {
	prevFile, prevTotal := maxFontFileBytes, maxArchiveBytes
	maxFontFileBytes = 1 << 20
	maxArchiveBytes = 12
	t.Cleanup(func() { maxFontFileBytes, maxArchiveBytes = prevFile, prevTotal })

	temp := t.TempDir()
	archivePath := filepath.Join(temp, "font.zip")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for _, name := range []string{"A.ttf", "B.ttf"} {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte("ten bytes!")); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	err = ExtractFontZip(archivePath, filepath.Join(temp, "out"))
	if err == nil {
		t.Fatal("ExtractFontZip() error = nil, want total-size error")
	}
	if !strings.Contains(err.Error(), "total uncompressed size") {
		t.Fatalf("ExtractFontZip() error = %v, want total-size error", err)
	}
}

func TestInstallInstallsMultipleFamilies(t *testing.T) {
	zipBytes := fontZip(t) // precomputed once; the transport runs on worker goroutines.
	client := &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       io.NopCloser(bytes.NewReader(zipBytes)),
			}, nil
		}),
	}

	destination := filepath.Join(t.TempDir(), "fonts")
	families := []string{"Hack", "JetBrainsMono", "FiraCode"}
	err := Install(t.Context(), Options{
		Release:     "latest",
		Destination: destination,
		Families:    families,
		HTTPClient:  client,
		Stderr:      io.Discard,
	})
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	for _, family := range families {
		entries, err := os.ReadDir(filepath.Join(destination, family))
		if err != nil || len(entries) == 0 {
			t.Fatalf("family %s not installed: entries = %d, err = %v", family, len(entries), err)
		}
	}
}

func TestInstallFailsWhenOneFamilyDownloadFails(t *testing.T) {
	zipBytes := fontZip(t)
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if strings.Contains(req.URL.Path, "Inter") {
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Status:     "404 Not Found",
					Body:       io.NopCloser(bytes.NewReader(nil)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       io.NopCloser(bytes.NewReader(zipBytes)),
			}, nil
		}),
	}

	err := Install(t.Context(), Options{
		Release:     "latest",
		Destination: filepath.Join(t.TempDir(), "fonts"),
		Families:    []string{"Hack", "Inter", "JetBrainsMono"},
		HTTPClient:  client,
		Stderr:      io.Discard,
	})
	if err == nil {
		t.Fatal("Install() error = nil, want failure for one family")
	}
	if !strings.Contains(err.Error(), "Inter") {
		t.Fatalf("Install() error = %v, want it to name the failing family Inter", err)
	}
}

func TestInstallVerifiesMatchingChecksum(t *testing.T) {
	zipBytes := fontZip(t)
	sum := sha256.Sum256(zipBytes)
	manifest := hex.EncodeToString(sum[:]) + "  Hack.zip\n"
	client := &http.Client{Transport: checksumRoutingTransport(manifest, zipBytes)}

	destination := filepath.Join(t.TempDir(), "fonts")
	err := Install(t.Context(), Options{
		Release:     "latest",
		Destination: destination,
		Families:    []string{"Hack"},
		HTTPClient:  client,
		Stderr:      io.Discard,
	})
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(destination, "Hack")); err != nil {
		t.Fatalf("family not installed: %v", err)
	}
}

func TestInstallRejectsChecksumMismatch(t *testing.T) {
	zipBytes := fontZip(t)
	manifest := strings.Repeat("a", 64) + "  Hack.zip\n" // deliberately wrong digest
	client := &http.Client{Transport: checksumRoutingTransport(manifest, zipBytes)}

	destination := filepath.Join(t.TempDir(), "fonts")
	err := Install(t.Context(), Options{
		Release:     "latest",
		Destination: destination,
		Families:    []string{"Hack"},
		HTTPClient:  client,
		Stderr:      io.Discard,
	})
	if err == nil {
		t.Fatal("Install() error = nil, want checksum mismatch")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("Install() error = %v, want checksum mismatch", err)
	}
	if _, err := os.Stat(filepath.Join(destination, "Hack")); !os.IsNotExist(err) {
		t.Fatalf("family must not be installed on mismatch, stat err = %v", err)
	}
}

// checksumRoutingTransport serves the SHA-256 manifest for the checksum URL and
// the zip bytes for any font download.
func checksumRoutingTransport(manifest string, zipBytes []byte) roundTripFunc {
	return func(req *http.Request) (*http.Response, error) {
		body := zipBytes
		if strings.HasSuffix(req.URL.Path, "SHA-256.txt") {
			body = []byte(manifest)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(bytes.NewReader(body)),
		}, nil
	}
}

func TestInstallReportsDownloadErrors(t *testing.T) {
	tests := []struct {
		name      string
		transport roundTripFunc
		wantSub   string
	}{
		{
			name: "non-2xx status",
			transport: func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Status:     "404 Not Found",
					Body:       io.NopCloser(bytes.NewReader(nil)),
				}, nil
			},
			wantSub: "404 Not Found",
		},
		{
			name: "transport error",
			transport: func(*http.Request) (*http.Response, error) {
				return nil, errTransport
			},
			wantSub: "download",
		},
		{
			name: "truncated body",
			transport: func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Body:       io.NopCloser(errReader{}),
				}, nil
			},
			wantSub: "copy download",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			destination := filepath.Join(t.TempDir(), "fonts")
			err := Install(t.Context(), Options{
				Release:     "latest",
				Destination: destination,
				Families:    []string{"Hack"},
				HTTPClient:  &http.Client{Transport: tt.transport},
			})
			if err == nil {
				t.Fatal("Install() error = nil, want download error")
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Fatalf("Install() error = %v, want substring %q", err, tt.wantSub)
			}
			if _, statErr := os.Stat(filepath.Join(destination, "Hack")); !os.IsNotExist(statErr) {
				t.Fatalf("family directory should not exist after failure, stat err = %v", statErr)
			}
		})
	}
}

func TestReplaceDirectoryRollsBackOnFailure(t *testing.T) {
	root := t.TempDir()
	destination := filepath.Join(root, "Hack")
	if err := os.MkdirAll(destination, 0o755); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(destination, "keep.ttf")
	if err := os.WriteFile(keep, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}

	// A source that does not exist forces the forward rename to fail after the
	// existing destination has been moved aside, exercising the rollback path.
	missingSource := filepath.Join(root, "does-not-exist")
	if err := replaceDirectory(missingSource, destination); err == nil {
		t.Fatal("replaceDirectory() error = nil, want rename failure")
	}

	data, err := os.ReadFile(keep)
	if err != nil || string(data) != "original" {
		t.Fatalf("rollback failed: original content = %q, err = %v", data, err)
	}
	if _, err := os.Stat(destination + ".old"); !os.IsNotExist(err) {
		t.Fatalf("backup should be restored (no .old left), stat err = %v", err)
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errTransport }

var errTransport = errorString("simulated network failure")

type errorString string

func (e errorString) Error() string { return string(e) }

// Unit tests for ChecksumURL.

func TestChecksumURLLatest(t *testing.T) {
	got := ChecksumURL("latest")
	want := "https://github.com/ryanoasis/nerd-fonts/releases/latest/download/SHA-256.txt"
	if got != want {
		t.Fatalf("ChecksumURL(\"latest\") = %q, want %q", got, want)
	}
}

func TestChecksumURLEmptyTreatedAsLatest(t *testing.T) {
	got := ChecksumURL("")
	want := "https://github.com/ryanoasis/nerd-fonts/releases/latest/download/SHA-256.txt"
	if got != want {
		t.Fatalf("ChecksumURL(\"\") = %q, want %q", got, want)
	}
}

func TestChecksumURLVersionedRelease(t *testing.T) {
	got := ChecksumURL("v3.4.0")
	want := "https://github.com/ryanoasis/nerd-fonts/releases/download/v3.4.0/SHA-256.txt"
	if got != want {
		t.Fatalf("ChecksumURL(\"v3.4.0\") = %q, want %q", got, want)
	}
}

func TestChecksumURLEscapesRelease(t *testing.T) {
	got := ChecksumURL("release candidate")
	want := "https://github.com/ryanoasis/nerd-fonts/releases/download/release%20candidate/SHA-256.txt"
	if got != want {
		t.Fatalf("ChecksumURL(\"release candidate\") = %q, want %q", got, want)
	}
}

// Unit tests for fetchChecksums.

func TestFetchChecksumsParsesTwoColumnFormat(t *testing.T) {
	manifest := "abc123  Hack.zip\ndef456  JetBrainsMono.ZIP\nskipped\n"
	client := &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       io.NopCloser(strings.NewReader(manifest)),
			}, nil
		}),
	}

	var stderr strings.Builder
	got := fetchChecksums(t.Context(), client, "v3.4.0", &stderr)

	if got["Hack"] != "abc123" {
		t.Fatalf("checksums[Hack] = %q, want abc123", got["Hack"])
	}
	if got["JetBrainsMono"] != "def456" {
		t.Fatalf("checksums[JetBrainsMono] = %q, want def456", got["JetBrainsMono"])
	}
	if _, ok := got["skipped"]; ok {
		t.Fatal("single-column line should not produce an entry")
	}
}

func TestFetchChecksumsNormalizesDigestToLower(t *testing.T) {
	manifest := "ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789  Hack.zip\n"
	client := &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       io.NopCloser(strings.NewReader(manifest)),
			}, nil
		}),
	}
	got := fetchChecksums(t.Context(), client, "latest", &strings.Builder{})
	for _, v := range got {
		if v != strings.ToLower(v) {
			t.Fatalf("digest %q was not lowercased", v)
		}
	}
}

func TestFetchChecksumsReturnsNilOnNon2xx(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Status:     "404 Not Found",
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		}),
	}
	var stderr strings.Builder
	got := fetchChecksums(t.Context(), client, "latest", &stderr)
	if got != nil {
		t.Fatalf("fetchChecksums() on 404 = %v, want nil (best-effort)", got)
	}
	if !strings.Contains(stderr.String(), "unavailable") {
		t.Fatalf("stderr = %q, want unavailable warning", stderr.String())
	}
}

func TestFetchChecksumsReturnsNilOnTransportError(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errTransport
		}),
	}
	var stderr strings.Builder
	got := fetchChecksums(t.Context(), client, "latest", &stderr)
	if got != nil {
		t.Fatalf("fetchChecksums() on transport error = %v, want nil", got)
	}
	if !strings.Contains(stderr.String(), "unavailable") {
		t.Fatalf("stderr = %q, want unavailable warning", stderr.String())
	}
}

func TestFetchChecksumsIgnoresNonZipLines(t *testing.T) {
	manifest := "aaa111  Hack.tar.gz\nbbb222  Hack.zip\nccc333  README.md\n"
	client := &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       io.NopCloser(strings.NewReader(manifest)),
			}, nil
		}),
	}
	got := fetchChecksums(t.Context(), client, "v3.4.0", &strings.Builder{})
	if len(got) != 1 || got["Hack"] != "bbb222" {
		t.Fatalf("fetchChecksums() = %v, want only Hack=bbb222", got)
	}
}

// Unit tests for syncWriter.

func TestSyncWriterSerializesWrites(t *testing.T) {
	// Verify that concurrent writes do not panic and all bytes are captured.
	var buf bytes.Buffer
	sw := &syncWriter{w: &buf}

	const workers = 20
	done := make(chan struct{})
	for range workers {
		go func() {
			defer func() { done <- struct{}{} }()
			for range 50 {
				if _, err := sw.Write([]byte("x")); err != nil {
					t.Errorf("syncWriter.Write() error = %v", err)
					return
				}
			}
		}()
	}
	for range workers {
		<-done
	}
	if buf.Len() != workers*50 {
		t.Fatalf("syncWriter captured %d bytes, want %d", buf.Len(), workers*50)
	}
}

// Unit tests for dedupeFamilies.

func TestDedupeFamiliesRemovesDuplicates(t *testing.T) {
	got := dedupeFamilies([]string{"Hack", "JetBrainsMono", "Hack", "FiraCode", "JetBrainsMono"})
	want := []string{"Hack", "JetBrainsMono", "FiraCode"}
	if len(got) != len(want) {
		t.Fatalf("dedupeFamilies() len = %d, want %d; got %v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Fatalf("dedupeFamilies()[%d] = %q, want %q", i, got[i], w)
		}
	}
}

func TestDedupeFamiliesPreservesOrder(t *testing.T) {
	input := []string{"Z", "A", "M", "A", "Z"}
	got := dedupeFamilies(input)
	want := []string{"Z", "A", "M"}
	if len(got) != len(want) || got[0] != "Z" || got[1] != "A" || got[2] != "M" {
		t.Fatalf("dedupeFamilies() = %v, want order preserved: %v", got, want)
	}
}

func TestDedupeFamiliesReturnsEmptyForEmpty(t *testing.T) {
	got := dedupeFamilies(nil)
	if len(got) != 0 {
		t.Fatalf("dedupeFamilies(nil) = %v, want empty", got)
	}
}

func TestDedupeFamiliesDoesNotMutateInput(t *testing.T) {
	input := []string{"Hack", "Hack"}
	_ = dedupeFamilies(input)
	if len(input) != 2 {
		t.Fatal("dedupeFamilies mutated its input slice")
	}
}

// Regression: install with duplicate families should install each family once.
func TestInstallDeduplicatesFamilies(t *testing.T) {
	zipBytes := fontZip(t)
	calls := 0
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if strings.HasSuffix(req.URL.Path, "SHA-256.txt") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Body:       io.NopCloser(strings.NewReader("")),
				}, nil
			}
			calls++
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       io.NopCloser(bytes.NewReader(zipBytes)),
			}, nil
		}),
	}

	err := Install(t.Context(), Options{
		Release:     "latest",
		Destination: filepath.Join(t.TempDir(), "fonts"),
		Families:    []string{"Hack", "Hack", "Hack"},
		HTTPClient:  client,
		Stderr:      io.Discard,
	})
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("download calls = %d, want 1 (duplicates collapsed)", calls)
	}
}

// Boundary: download that exceeds maxDownloadBytes via Content-Length header.
func TestInstallRejectsOversizeDownloadViaContentLength(t *testing.T) {
	prev := maxDownloadBytes
	maxDownloadBytes = 10
	t.Cleanup(func() { maxDownloadBytes = prev })

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if strings.HasSuffix(req.URL.Path, "SHA-256.txt") {
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Status:     "404 Not Found",
					Body:       io.NopCloser(strings.NewReader("")),
				}, nil
			}
			return &http.Response{
				StatusCode:    http.StatusOK,
				Status:        "200 OK",
				ContentLength: 9999999,
				Body:          io.NopCloser(strings.NewReader("")),
			}, nil
		}),
	}

	err := Install(t.Context(), Options{
		Release:     "latest",
		Destination: filepath.Join(t.TempDir(), "fonts"),
		Families:    []string{"Hack"},
		HTTPClient:  client,
		Stderr:      io.Discard,
	})
	if err == nil {
		t.Fatal("Install() error = nil, want oversize rejection via Content-Length")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("Install() error = %v, want size-limit message", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func fontZip(t *testing.T) []byte {
	t.Helper()

	var body bytes.Buffer
	writer := zip.NewWriter(&body)
	entry, err := writer.Create("Hack.ttf")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("font")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return body.Bytes()
}

func noFontZip(t *testing.T) []byte {
	t.Helper()

	var body bytes.Buffer
	writer := zip.NewWriter(&body)
	entry, err := writer.Create("README.md")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("docs")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return body.Bytes()
}
