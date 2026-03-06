package harness_test

import (
	"testing"

	"h2/internal/config"
	"h2/internal/session/agent/harness"

	// Register harness implementations.
	_ "h2/internal/session/agent/harness/claude"
	_ "h2/internal/session/agent/harness/codex"
	_ "h2/internal/session/agent/harness/generic"
)

func TestResolve_ClaudeCode(t *testing.T) {
	h, err := harness.Resolve(&config.RuntimeConfig{HarnessType: "claude_code"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h == nil {
		t.Fatal("expected non-nil harness")
	}
	if h.Name() != "claude_code" {
		t.Errorf("Name() = %q, want %q", h.Name(), "claude_code")
	}
}

func TestResolve_Codex(t *testing.T) {
	h, err := harness.Resolve(&config.RuntimeConfig{HarnessType: "codex"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h == nil {
		t.Fatal("expected non-nil harness")
	}
	if h.Name() != "codex" {
		t.Errorf("Name() = %q, want %q", h.Name(), "codex")
	}
	if h.Command() != "codex" {
		t.Errorf("Command() = %q, want %q", h.Command(), "codex")
	}
}

func TestResolve_Generic(t *testing.T) {
	h, err := harness.Resolve(&config.RuntimeConfig{HarnessType: "generic", Command: "bash"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h == nil {
		t.Fatal("expected non-nil harness")
	}
	if h.Name() != "generic" {
		t.Errorf("Name() = %q, want %q", h.Name(), "generic")
	}
	if h.Command() != "bash" {
		t.Errorf("Command() = %q, want %q", h.Command(), "bash")
	}
}

func TestResolve_Generic_NoCommand(t *testing.T) {
	_, err := harness.Resolve(&config.RuntimeConfig{HarnessType: "generic"}, nil)
	if err == nil {
		t.Fatal("expected error for generic without command")
	}
}

func TestResolveNativeSessionLogPath_Claude(t *testing.T) {
	rc := &config.RuntimeConfig{
		HarnessType:             "claude_code",
		HarnessConfigPathPrefix: "/home/user/.h2/claude-config",
		Profile:                 "default",
		CWD:                     "/Users/dcosson/projects/h2",
		HarnessSessionID:        "abc-123",
	}
	got := harness.ResolveNativeSessionLogPath(rc)
	want := "/home/user/.h2/claude-config/default/projects/-Users-dcosson-projects-h2/abc-123.jsonl"
	if got != want {
		t.Errorf("ResolveNativeSessionLogPath() = %q, want %q", got, want)
	}
}

func TestResolveNativeSessionLogPath_Codex(t *testing.T) {
	rc := &config.RuntimeConfig{
		HarnessType:             "codex",
		HarnessConfigPathPrefix: "/home/user/.h2/codex-config",
		Profile:                 "default",
		CWD:                     "/tmp",
		HarnessSessionID:        "abc-123",
	}
	got := harness.ResolveNativeSessionLogPath(rc)
	if got != "" {
		t.Errorf("ResolveNativeSessionLogPath() for codex = %q, want empty", got)
	}
}

func TestResolveNativeSessionLogPath_Generic(t *testing.T) {
	rc := &config.RuntimeConfig{
		HarnessType: "generic",
		Command:     "vim",
		CWD:         "/tmp",
	}
	got := harness.ResolveNativeSessionLogPath(rc)
	if got != "" {
		t.Errorf("ResolveNativeSessionLogPath() for generic = %q, want empty", got)
	}
}

func TestResolveNativeSessionLogPath_UnknownHarness(t *testing.T) {
	rc := &config.RuntimeConfig{
		HarnessType: "unknown_harness",
		CWD:         "/tmp",
	}
	got := harness.ResolveNativeSessionLogPath(rc)
	if got != "" {
		t.Errorf("ResolveNativeSessionLogPath() for unknown = %q, want empty", got)
	}
}

func TestResolve_ClaudeCode_ConfigPassthrough(t *testing.T) {
	rc := &config.RuntimeConfig{
		HarnessType:             "claude_code",
		HarnessConfigPathPrefix: "/tmp/test",
		Profile:                 "config",
		Model:                   "opus",
	}
	h, err := harness.Resolve(rc, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.Command() != "claude" {
		t.Errorf("Command() = %q, want %q", h.Command(), "claude")
	}
	if h.DisplayCommand() != "claude" {
		t.Errorf("DisplayCommand() = %q, want %q", h.DisplayCommand(), "claude")
	}
	// Verify the config was passed through by checking BuildCommandEnvVars.
	envVars := h.BuildCommandEnvVars("/unused")
	if envVars["CLAUDE_CONFIG_DIR"] != "/tmp/test/config" {
		t.Errorf("CLAUDE_CONFIG_DIR = %q, want %q", envVars["CLAUDE_CONFIG_DIR"], "/tmp/test/config")
	}
}
