package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/w0rxbend/nerd-font-installer/internal/nerdfonts"
)

func TestModelSelectsReleaseAndFamilies(t *testing.T) {
	m := newModel([]nerdfonts.Release{
		{
			Name:     "v3.4.0",
			TagName:  "v3.4.0",
			Families: []string{"Hack", "JetBrainsMono"},
		},
	}, "/tmp/fonts", true)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(model)
	if m.step != stepFamilies {
		t.Fatalf("step = %v, want stepFamilies", m.step)
	}
	if m.selectedRelease.TagName != "v3.4.0" {
		t.Fatalf("selectedRelease = %q", m.selectedRelease.TagName)
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = next.(model)
	if m.selectedCount() != 1 {
		t.Fatalf("selectedCount() = %d, want 1", m.selectedCount())
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	m = next.(model)
	if m.selectedCount() != 2 {
		t.Fatalf("selectedCount() = %d, want 2", m.selectedCount())
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	m = next.(model)
	if m.selectedCount() != 0 {
		t.Fatalf("selectedCount() = %d, want 0", m.selectedCount())
	}
}
