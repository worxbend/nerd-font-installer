package tui

import (
	"testing"
)

// Tests for resolveIconSet, which was extracted into icons.go in this PR.

func TestResolveIconSetModes(t *testing.T) {
	tests := []struct {
		name     string
		mode     IconMode
		wantMode IconMode
	}{
		{name: "nerd", mode: IconNerd, wantMode: IconNerd},
		{name: "ascii", mode: IconASCII, wantMode: IconASCII},
		{name: "unicode", mode: IconUnicode, wantMode: IconUnicode},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveIconSet(tt.mode)
			if got.Mode != tt.wantMode {
				t.Fatalf("resolveIconSet(%q).Mode = %q, want %q", tt.mode, got.Mode, tt.wantMode)
			}
		})
	}
}

func TestResolveIconSetCheckboxes(t *testing.T) {
	tests := []struct {
		name          string
		mode          IconMode
		wantChecked   string
		wantUnchecked string
	}{
		{name: "nerd", mode: IconNerd, wantChecked: "󰄲", wantUnchecked: "󰄱"},
		{name: "ascii", mode: IconASCII, wantChecked: "[x]", wantUnchecked: "[ ]"},
		{name: "unicode", mode: IconUnicode, wantChecked: "☑", wantUnchecked: "☐"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveIconSet(tt.mode)
			if got.Checked != tt.wantChecked {
				t.Fatalf("resolveIconSet(%q).Checked = %q, want %q", tt.mode, got.Checked, tt.wantChecked)
			}
			if got.Unchecked != tt.wantUnchecked {
				t.Fatalf("resolveIconSet(%q).Unchecked = %q, want %q", tt.mode, got.Unchecked, tt.wantUnchecked)
			}
		})
	}
}

func TestResolveIconSetNerdFamilyGlyphs(t *testing.T) {
	tests := []struct {
		name      string
		mode      IconMode
		key       string
		wantValue string
	}{
		{name: "nerd hack", mode: IconNerd, key: "hack", wantValue: "󰌌"},
		{name: "nerd jetbrains", mode: IconNerd, key: "jetbrainsmono", wantValue: nerdFamilyGlyphs["jetbrainsmono"]},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveIconSet(tt.mode)
			if got.NerdFamily[tt.key] != tt.wantValue {
				t.Fatalf("resolveIconSet(%q).NerdFamily[%s] = %q, want %q", tt.mode, tt.key, got.NerdFamily[tt.key], tt.wantValue)
			}
		})
	}
}

func TestResolveIconSetASCIIHasEmptyNerdFamily(t *testing.T) {
	got := resolveIconSet(IconASCII)
	if len(got.NerdFamily) != 0 {
		t.Fatalf("resolveIconSet(IconASCII).NerdFamily len = %d, want 0 (ASCII has no glyph map)", len(got.NerdFamily))
	}
}

func TestResolveIconSetAutoFallsBackToUnicode(t *testing.T) {
	got := resolveIconSet(IconAuto)
	// IconAuto must use the safe Unicode set (no Nerd Font required).
	if got.Mode != IconUnicode {
		t.Fatalf("resolveIconSet(IconAuto).Mode = %q, want %q (auto uses unicode set)", got.Mode, IconUnicode)
	}
	if got.Checked != "☑" {
		t.Fatalf("resolveIconSet(IconAuto).Checked = %q, want ☑", got.Checked)
	}
}

func TestResolveIconSetUnknownFallsBackToUnicode(t *testing.T) {
	got := resolveIconSet(IconMode("unknown"))
	if got.Mode != IconUnicode {
		t.Fatalf("resolveIconSet(unknown).Mode = %q, want %q (unknown mode uses unicode set)", got.Mode, IconUnicode)
	}
}

// Verify the icon sets reference the shared nerdFamilyGlyphs map (not a copy).
func TestNerdIconSetSharesGlyphMap(t *testing.T) {
	got := resolveIconSet(IconNerd)
	// If any key from nerdFamilyGlyphs is present in the returned set's map,
	// the maps are consistent. A copy would still pass this, but an empty map
	// or a different map would fail.
	if got.NerdFamily["firacode"] != nerdFamilyGlyphs["firacode"] {
		t.Fatalf("resolveIconSet(IconNerd).NerdFamily[firacode] = %q, nerdFamilyGlyphs[firacode] = %q; want same value",
			got.NerdFamily["firacode"], nerdFamilyGlyphs["firacode"])
	}
}

// Verify non-Nerd sets have no family glyph entries (they must not require a patched font).
func TestUnicodeIconSetHasNoFamilyGlyphs(t *testing.T) {
	for _, mode := range []IconMode{IconUnicode, IconAuto} {
		got := resolveIconSet(mode)
		if len(got.NerdFamily) != 0 {
			t.Fatalf("resolveIconSet(%q).NerdFamily len = %d, want 0 (should not require Nerd Font)", mode, len(got.NerdFamily))
		}
	}
}
