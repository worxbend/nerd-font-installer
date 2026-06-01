package nerdfonts

import (
	"reflect"
	"testing"
)

func TestFamiliesFromAssets(t *testing.T) {
	got := familiesFromAssets([]string{
		"JetBrainsMono.zip",
		"README.md",
		"Hack.ZIP",
		"JetBrainsMono.zip",
		"SymbolsOnly.tar.xz",
	})
	want := []string{"Hack", "JetBrainsMono"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("familiesFromAssets() = %#v, want %#v", got, want)
	}
}

func TestWithPage(t *testing.T) {
	got, err := withPage("https://example.test/releases?existing=1", 3)
	if err != nil {
		t.Fatalf("withPage() error = %v", err)
	}
	want := "https://example.test/releases?existing=1&page=3&per_page=100"
	if got != want {
		t.Fatalf("withPage() = %q, want %q", got, want)
	}
}
