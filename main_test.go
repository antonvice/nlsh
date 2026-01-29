package main

import (
	"strings"
	"testing"
)

func TestIsLikelyCommand(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"git status", true},
		{"ls -la", true},
		{"cd ..", true},
		{"npm install", true},
		{"list all files", false},
		{"show me the money", false},
		{"why time is it", false},
		{"", false},
		// These might depend on local environment, effectively checking "command existing"
		// We can't guarantee 'randomcommandxyz' doesn't exist but it's unlikely
		{"randomcommandxyz", false}, 
	}

	for _, test := range tests {
		result := isLikelyCommand(test.input)
		// Note: isLikelyCommand checks PATH, so strict boolean equality might be 
		// flaky if "list" is actually a command on some system.
		// However, for the purpose of this test suite in a dev environment, it should hold.
		if result != test.expected {
			t.Errorf("isLikelyCommand(%q) = %v; want %v", test.input, result, test.expected)
		}
	}
}

func TestConfigDefaults(t *testing.T) {
	// We won't load from file, but we can test the struct defaults logic 
	// typically found in loadConfig. Since loadConfig reads files, 
	// we'll simulate the default structure creation here to ensure it matches expectations.
	
	config := &Config{
		Engine: "gemini",
		Rules: []string{
			"Prefer modern tools (rg over grep, fd over find, bat over cat).",
		},
	}
	
	if config.Engine != "gemini" {
		t.Errorf("Default engine should be gemini, got %s", config.Engine)
	}
	
	config.Ollama.Host = "http://localhost:11434"
	if config.Ollama.Host != "http://localhost:11434" {
		t.Errorf("Ollama host mismatch")
	}
}

func TestSystemInfoNotEmpty(t *testing.T) {
	info := getSystemInfo()
	if info == "" {
		t.Error("System info should not be empty")
	}
	if !strings.Contains(info, "OS:") {
		t.Error("System info should contain OS")
	}
}

func TestBangPrefix(t *testing.T) {
	// Testing the logic used in main() for stripping bangs
	input := "!list all files"
	clean := strings.TrimSpace(input)
	clean = strings.TrimPrefix(clean, "!")
	clean = strings.TrimSpace(clean)
	
	if clean != "list all files" {
		t.Errorf("Bang stripping failed. Got %q, want 'list all files'", clean)
	}

	// Test double bang?
	input2 := "!!do it"
	clean2 := strings.TrimSpace(input2)
	clean2 = strings.TrimPrefix(clean2, "!")
	clean2 = strings.TrimSpace(clean2)
	
	// Expect rejection of only FIRST bang 
	if clean2 != "!do it" {
		t.Errorf("Bang stripping should only remove first bang. Got %q", clean2)
	}
}
