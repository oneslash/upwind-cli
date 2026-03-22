package config

import "testing"

func TestResolveDefaults(t *testing.T) {
	runtime, err := Resolve(Options{})
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}

	if runtime.Region != "us" {
		t.Fatalf("expected default region us, got %s", runtime.Region)
	}
	if runtime.BaseURL != "https://api.upwind.io" {
		t.Fatalf("unexpected base URL: %s", runtime.BaseURL)
	}
	if runtime.AuthURL != "https://auth.upwind.io" {
		t.Fatalf("unexpected auth URL: %s", runtime.AuthURL)
	}
	if runtime.Audience != "https://api.upwind.io" {
		t.Fatalf("unexpected audience: %s", runtime.Audience)
	}
	if runtime.Output != "table" {
		t.Fatalf("unexpected default output: %s", runtime.Output)
	}
}

func TestResolveRejectsUnsupportedOutput(t *testing.T) {
	if _, err := Resolve(Options{Output: "yaml"}); err == nil {
		t.Fatal("expected unsupported output format error")
	}
}
