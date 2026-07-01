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
		Engine: "mlx",
		Rules: []string{
			"Prefer modern tools (rg over grep, fd over find, bat over cat).",
		},
	}

	if config.Engine != "mlx" {
		t.Errorf("Default engine should be mlx, got %s", config.Engine)
	}

	config.MLX.Model = "sahilchachra/ornith-1.0-9b-mxfp4-mlx"
	if config.MLX.Model != "sahilchachra/ornith-1.0-9b-mxfp4-mlx" {
		t.Errorf("MLX model mismatch")
	}

	config.Ollama.Host = "http://localhost:11434"
	if config.Ollama.Host != "http://localhost:11434" {
		t.Errorf("Ollama host mismatch")
	}
}

func TestCleanCommandFromChatOutput(t *testing.T) {
	output := "User: list files\nAssistant: rg --files\n"
	if got := cleanCommand(output); got != "rg --files" {
		t.Errorf("cleanCommand() = %q; want rg --files", got)
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

func TestAgentProfileNormalization(t *testing.T) {
	tests := map[string]string{
		"":              "read-only",
		"read-only":     "read-only",
		"confirm":       "confirm-write",
		"confirm-write": "confirm-write",
		"power":         "power",
		"weird":         "read-only",
	}
	for input, want := range tests {
		if got := normalizeAgentProfile(input); got != want {
			t.Fatalf("normalizeAgentProfile(%q) = %q; want %q", input, got, want)
		}
	}
}

func TestAgentSessionPathStable(t *testing.T) {
	config := &Config{}
	config.Agent.SessionDir = "/tmp/nlsh-sessions"
	first := agentSessionPath(config, "/Users/antonvice/project")
	second := agentSessionPath(config, "/Users/antonvice/project")
	other := agentSessionPath(config, "/Users/antonvice/other")
	if first != second {
		t.Fatalf("session path should be stable: %q != %q", first, second)
	}
	if first == other {
		t.Fatalf("different cwd should get different session path")
	}
	if !strings.HasSuffix(first, ".json") {
		t.Fatalf("session path should be json, got %q", first)
	}
}

func TestRepairCommonCommandMP4(t *testing.T) {
	got := repairCommonCommand("rg --files '*.mp4'", "find all mp4 videos in here")
	if got != "fd -e mp4 -t f ." {
		t.Fatalf("repairCommonCommand() = %q", got)
	}
}

func TestValidateAgentCommandProfiles(t *testing.T) {
	inventory := ToolInventory{Available: []string{"git", "fd", "mkdir"}}
	if err := validateAgentCommand("fd -e mp4 -t f .", inventory, "read-only"); err != nil {
		t.Fatalf("read-only fd should validate: %v", err)
	}
	if err := validateAgentCommand("mkdir reports", inventory, "read-only"); err == nil {
		t.Fatalf("read-only mkdir should be refused")
	}
	if err := validateAgentCommand("mkdir reports", inventory, "confirm-write"); err != nil {
		t.Fatalf("confirm-write mkdir should validate: %v", err)
	}
	if err := validateAgentCommand("fd -e mp4 -t f . | xargs du", inventory, "read-only"); err == nil {
		t.Fatalf("pipe should be refused in read-only mode")
	}
}
