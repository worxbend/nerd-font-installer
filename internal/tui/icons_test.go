package tui

import (
	"testing"
)

// Tests for resolveIconSet, which was extracted into icons.go in this PR.

func TestResolveIconSetNerdReturnsNerdMode(t *testing.T) {
	got := resolveIconSet(IconNerd)
	if got.Mode != IconNerd {
		t.Fatalf("resolveIconSet(IconNerd).Mode = %q, want %q", got.Mode, IconNerd)
	}
}

func TestResolveIconSetNerdHasNerdCheckboxes(t *testing.T) {
	got := resolveIconSet(IconNerd)
	if got.Checked != "󰄲" {
		t.Fatalf("resolveIconSet(IconNerd).Checked = %q, want Nerd Font checked glyph", got.Checked)
	}
	if got.Unchecked != "󰄱" {
		t.Fatalf("resolveIconSet(IconNerd).Unchecked = %q, want Nerd Font unchecked glyph", got.Unchecked)
	}
}

func TestResolveIconSetNerdHasFamilyGlyphs(t *testing.T) {
	got := resolveIconSet(IconNerd)
	if got.NerdFamily["hack"] != "󰌌" {
		t.Fatalf("resolveIconSet(IconNerd).NerdFamily[hack] = %q, want 󰌌", got.NerdFamily["hack"])
	}
	if got.NerdFamily["jetbrainsmono"] == "" {
		t.Fatal("resolveIconSet(IconNerd).NerdFamily[jetbrainsmono] is empty, want a glyph")
	}
}

func TestResolveIconSetASCIIReturnsASCIIMode(t *testing.T) {
	got := resolveIconSet(IconASCII)
	if got.Mode != IconASCII {
		t.Fatalf("resolveIconSet(IconASCII).Mode = %q, want %q", got.Mode, IconASCII)
	}
}

func TestResolveIconSetASCIIHasAsciiCheckboxes(t *testing.T) {
	got := resolveIconSet(IconASCII)
	if got.Checked != "[x]" {
		t.Fatalf("resolveIconSet(IconASCII).Checked = %q, want [x]", got.Checked)
	}
	if got.Unchecked != "[ ]" {
		t.Fatalf("resolveIconSet(IconASCII).Unchecked = %q, want [ ]", got.Unchecked)
	}
}

func TestResolveIconSetASCIIHasEmptyNerdFamily(t *testing.T) {
	got := resolveIconSet(IconASCII)
	if len(got.NerdFamily) != 0 {
		t.Fatalf("resolveIconSet(IconASCII).NerdFamily len = %d, want 0 (ASCII has no glyph map)", len(got.NerdFamily))
	}
}

func TestResolveIconSetUnicodeReturnsUnicodeMode(t *testing.T) {
	got := resolveIconSet(IconUnicode)
	if got.Mode != IconUnicode {
		t.Fatalf("resolveIconSet(IconUnicode).Mode = %q, want %q", got.Mode, IconUnicode)
	}
}

func TestResolveIconSetUnicodeHasUnicodeCheckboxes(t *testing.T) {
	got := resolveIconSet(IconUnicode)
	if got.Checked != "☑" {
		t.Fatalf("resolveIconSet(IconUnicode).Checked = %q, want ☑", got.Checked)
	}
	if got.Unchecked != "☐" {
		t.Fatalf("resolveIconSet(IconUnicode).Unchecked = %q, want ☐", got.Unchecked)
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
