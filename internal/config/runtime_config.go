package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RuntimeConfig is the fully-resolved, serialized configuration for a daemon
// session. It is written to <session-dir>/session.metadata.json by the launcher
// and read by the daemon on startup. It serves as both the daemon's input
// config and the persistent session record for resume and tooling (peek, stats).
type RuntimeConfig struct {
	// Identity & provenance.
	AgentName string `json:"agent_name"`
	SessionID string `json:"session_id"`
	RoleName  string `json:"role,omitempty"`
	Pod       string `json:"pod,omitempty"`

	// Harness configuration.
	HarnessType      string `json:"harness_type"`
	HarnessConfigDir string `json:"harness_config_dir,omitempty"`
	// HarnessSessionID is the session ID as known by the underlying harness
	// (e.g. Claude Code's session UUID, Codex's conversation.id). For harnesses
	// that accept a session-id input arg (currently just Claude Code), this will
	// equal SessionID since h2 generates the UUID and passes it through. For
	// other harnesses (Codex, future), this is reported async after launch and
	// will differ from SessionID. Use HarnessSessionID to look up session logs
	// in the harness's own config directory.
	HarnessSessionID string   `json:"harness_session_id,omitempty"`
	Command          string   `json:"command"`
	Args             []string `json:"args,omitempty"`
	Model            string   `json:"model,omitempty"`

	// Working directory.
	CWD string `json:"cwd"`

	// Prompt configuration.
	Instructions string `json:"instructions,omitempty"`
	SystemPrompt string `json:"system_prompt,omitempty"`

	// Permission configuration.
	ClaudePermissionMode string `json:"claude_permission_mode,omitempty"`
	CodexSandboxMode     string `json:"codex_sandbox_mode,omitempty"`
	CodexAskForApproval  string `json:"codex_ask_for_approval,omitempty"`

	// Additional directories.
	AdditionalDirs []string `json:"additional_dirs,omitempty"`

	// Heartbeat nudge configuration.
	HeartbeatIdleTimeout string `json:"heartbeat_idle_timeout,omitempty"` // Go duration string
	HeartbeatMessage     string `json:"heartbeat_message,omitempty"`
	HeartbeatCondition   string `json:"heartbeat_condition,omitempty"`

	// Overrides (recorded for display/debugging).
	Overrides map[string]string `json:"overrides,omitempty"`

	// Resume support.
	ResumeSessionID string `json:"resume_session_id,omitempty"`

	// Timestamps.
	StartedAt string `json:"started_at"`

	// --- Deprecated fields for backward compatibility on read ---
	// ClaudeConfigDir was the old field name; migrated to HarnessConfigDir on load.
	ClaudeConfigDir string `json:"claude_config_dir,omitempty"`
}

const runtimeConfigFilename = "session.metadata.json"

// WriteRuntimeConfig atomically writes the RuntimeConfig to the session directory.
// Uses write-to-temp + rename to prevent corruption from concurrent readers.
func WriteRuntimeConfig(sessionDir string, rc *RuntimeConfig) error {
	if sessionDir == "" {
		return nil
	}
	if rc.StartedAt == "" {
		rc.StartedAt = time.Now().UTC().Format(time.RFC3339)
	}
	data, err := json.MarshalIndent(rc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal runtime config: %w", err)
	}
	data = append(data, '\n')
	path := filepath.Join(sessionDir, runtimeConfigFilename)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write runtime config tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp) // best-effort cleanup
		return fmt.Errorf("rename runtime config: %w", err)
	}
	return nil
}

// ReadRuntimeConfig reads and validates the RuntimeConfig from a session directory.
// Performs deprecated field migration and strict validation of required fields.
func ReadRuntimeConfig(sessionDir string) (*RuntimeConfig, error) {
	path := filepath.Join(sessionDir, runtimeConfigFilename)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rc RuntimeConfig
	if err := json.Unmarshal(data, &rc); err != nil {
		return nil, fmt.Errorf("parse runtime config: %w", err)
	}

	// One-time migrations for renamed fields.
	if rc.HarnessConfigDir == "" && rc.ClaudeConfigDir != "" {
		rc.HarnessConfigDir = rc.ClaudeConfigDir
		rc.ClaudeConfigDir = ""
	}

	if err := rc.Validate(); err != nil {
		return nil, err
	}

	return &rc, nil
}

// runtimeConfigRequiredFields lists fields that must be non-empty for a valid RuntimeConfig.
var runtimeConfigRequiredFields = []struct {
	name  string
	value func(*RuntimeConfig) string
}{
	{"agent_name", func(rc *RuntimeConfig) string { return rc.AgentName }},
	{"session_id", func(rc *RuntimeConfig) string { return rc.SessionID }},
	{"harness_type", func(rc *RuntimeConfig) string { return rc.HarnessType }},
	{"command", func(rc *RuntimeConfig) string { return rc.Command }},
	{"cwd", func(rc *RuntimeConfig) string { return rc.CWD }},
	{"started_at", func(rc *RuntimeConfig) string { return rc.StartedAt }},
}

// Validate checks that all required fields are present and non-empty.
func (rc *RuntimeConfig) Validate() error {
	var missing []string
	for _, f := range runtimeConfigRequiredFields {
		if f.value(rc) == "" {
			missing = append(missing, f.name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("invalid runtime config: missing required fields: %s", strings.Join(missing, ", "))
	}
	return nil
}

// ParseHeartbeatIdleTimeout parses the HeartbeatIdleTimeout string as a Go duration.
// Returns zero duration if the field is empty.
func (rc *RuntimeConfig) ParseHeartbeatIdleTimeout() (time.Duration, error) {
	if rc.HeartbeatIdleTimeout == "" {
		return 0, nil
	}
	return time.ParseDuration(rc.HeartbeatIdleTimeout)
}
