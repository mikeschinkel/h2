package session

import (
	"testing"

	"h2/internal/config"
	"h2/internal/session/agent/harness"
)

func TestRuntimeConfig_FieldsStoredOnSession(t *testing.T) {
	// Verify that RuntimeConfig fields are accessible through the session.
	rc := &config.RuntimeConfig{
		AgentName:    "test-agent",
		SessionID:    "test-uuid",
		Command:      "claude",
		Instructions: "You are a test agent.\nDo test things.",
		HarnessType:  "claude_code",
		CWD:          "/tmp",
		StartedAt:    "2024-01-01T00:00:00Z",
	}

	s := NewFromConfig(rc)
	h, err := harness.Resolve(rc, nil)
	if err != nil {
		t.Fatalf("resolve harness: %v", err)
	}
	s.harness = h

	if s.RC.Instructions != "You are a test agent.\nDo test things." {
		t.Fatalf("Instructions not stored on RC: got %q", s.RC.Instructions)
	}

	// Verify childArgs includes --append-system-prompt.
	args := s.childArgs()
	found := false
	for i, arg := range args {
		if arg == "--append-system-prompt" && i+1 < len(args) {
			found = true
			if args[i+1] != rc.Instructions {
				t.Fatalf("--append-system-prompt value = %q, want %q", args[i+1], rc.Instructions)
			}
		}
	}
	if !found {
		t.Fatal("childArgs should include --append-system-prompt when Instructions is set")
	}
}

func TestRuntimeConfig_EmptyInstructionsNotInChildArgs(t *testing.T) {
	rc := &config.RuntimeConfig{
		AgentName:   "test-agent",
		SessionID:   "test-uuid",
		Command:     "claude",
		HarnessType: "claude_code",
		CWD:         "/tmp",
		StartedAt:   "2024-01-01T00:00:00Z",
	}

	s := NewFromConfig(rc)
	h, err := harness.Resolve(rc, nil)
	if err != nil {
		t.Fatalf("resolve harness: %v", err)
	}
	s.harness = h

	// Verify childArgs does NOT include --append-system-prompt.
	args := s.childArgs()
	for _, arg := range args {
		if arg == "--append-system-prompt" {
			t.Fatal("childArgs should NOT include --append-system-prompt when Instructions is empty")
		}
	}
}

func TestRuntimeConfig_AllFieldsStoredOnSession(t *testing.T) {
	rc := &config.RuntimeConfig{
		AgentName:            "test-agent",
		SessionID:            "test-uuid",
		Command:              "claude",
		Instructions:         "Instructions here",
		SystemPrompt:         "Custom system prompt",
		Model:                "claude-opus-4-6",
		ClaudePermissionMode: "plan",
		HarnessType:          "claude_code",
		CWD:                  "/tmp",
		StartedAt:            "2024-01-01T00:00:00Z",
	}

	s := NewFromConfig(rc)
	h, err := harness.Resolve(rc, nil)
	if err != nil {
		t.Fatalf("resolve harness: %v", err)
	}
	s.harness = h

	if s.RC.SystemPrompt != "Custom system prompt" {
		t.Fatalf("SystemPrompt not stored: got %q", s.RC.SystemPrompt)
	}
	if s.RC.Model != "claude-opus-4-6" {
		t.Fatalf("Model not stored: got %q", s.RC.Model)
	}
	if s.RC.ClaudePermissionMode != "plan" {
		t.Fatalf("ClaudePermissionMode not stored: got %q", s.RC.ClaudePermissionMode)
	}

	// Verify all fields appear in childArgs.
	args := s.childArgs()
	expectPairs := map[string]string{
		"--system-prompt":        "Custom system prompt",
		"--append-system-prompt": "Instructions here",
		"--model":                "claude-opus-4-6",
		"--permission-mode":      "plan",
	}
	for flag, wantVal := range expectPairs {
		found := false
		for i, arg := range args {
			if arg == flag && i+1 < len(args) {
				found = true
				if args[i+1] != wantVal {
					t.Errorf("%s value = %q, want %q", flag, args[i+1], wantVal)
				}
			}
		}
		if !found {
			t.Errorf("expected %s in childArgs, not found. args: %v", flag, args)
		}
	}
}

func TestRuntimeConfig_CodexFields(t *testing.T) {
	rc := &config.RuntimeConfig{
		AgentName:           "test-agent",
		SessionID:           "test-uuid",
		Command:             "codex",
		Instructions:        "Do work",
		CodexAskForApproval: "never",
		CodexSandboxMode:    "danger-full-access",
		HarnessType:         "codex",
		CWD:                 "/tmp",
		StartedAt:           "2024-01-01T00:00:00Z",
	}

	s := NewFromConfig(rc)

	if s.RC.CodexAskForApproval != "never" {
		t.Fatalf("CodexAskForApproval not stored: got %q", s.RC.CodexAskForApproval)
	}
	if s.RC.CodexSandboxMode != "danger-full-access" {
		t.Fatalf("CodexSandboxMode not stored: got %q", s.RC.CodexSandboxMode)
	}
}
