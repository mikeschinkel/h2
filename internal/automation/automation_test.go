package automation

import (
	"testing"
	"time"

	"h2/internal/session/agent/monitor"
)

func TestActionValidate_ExecOnly(t *testing.T) {
	a := Action{Exec: "echo hello"}
	if err := a.Validate(); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}
}

func TestActionValidate_MessageOnly(t *testing.T) {
	a := Action{Message: "nudge"}
	if err := a.Validate(); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}
}

func TestActionValidate_Neither(t *testing.T) {
	a := Action{}
	if err := a.Validate(); err == nil {
		t.Fatal("expected error for empty action")
	}
}

func TestActionValidate_Both(t *testing.T) {
	a := Action{Exec: "echo", Message: "msg"}
	if err := a.Validate(); err == nil {
		t.Fatal("expected error for both exec and message")
	}
}

func TestActionValidate_BadPriority(t *testing.T) {
	a := Action{Exec: "echo", Priority: "bogus"}
	if err := a.Validate(); err == nil {
		t.Fatal("expected error for bad priority")
	}
}

func TestActionValidate_GoodPriority(t *testing.T) {
	for _, p := range []string{"interrupt", "normal", "idle-first", "idle"} {
		a := Action{Exec: "echo", Priority: p}
		if err := a.Validate(); err != nil {
			t.Fatalf("priority %q: %v", p, err)
		}
	}
}

func TestParseConditionMode(t *testing.T) {
	tests := []struct {
		input string
		want  ConditionMode
		ok    bool
	}{
		{"run_if", RunIf, true},
		{"", RunIf, true},
		{"stop_when", StopWhen, true},
		{"run_once_when", RunOnceWhen, true},
		{"bogus", 0, false},
	}
	for _, tt := range tests {
		got, ok := ParseConditionMode(tt.input)
		if ok != tt.ok || got != tt.want {
			t.Errorf("ParseConditionMode(%q) = (%v, %v), want (%v, %v)", tt.input, got, ok, tt.want, tt.ok)
		}
	}
}

func TestConditionModeString(t *testing.T) {
	if RunIf.String() != "run_if" {
		t.Errorf("RunIf.String() = %q", RunIf.String())
	}
	if StopWhen.String() != "stop_when" {
		t.Errorf("StopWhen.String() = %q", StopWhen.String())
	}
	if RunOnceWhen.String() != "run_once_when" {
		t.Errorf("RunOnceWhen.String() = %q", RunOnceWhen.String())
	}
}

func TestTrigger_MatchesEvent_StateChange(t *testing.T) {
	tr := &Trigger{
		Event: "state_change",
		State: "idle",
	}
	evt := monitor.AgentEvent{
		Type:      monitor.EventStateChange,
		Timestamp: time.Now(),
		Data:      monitor.StateChangeData{State: monitor.StateIdle, SubState: monitor.SubStateNone},
	}
	if !tr.MatchesEvent(evt) {
		t.Fatal("expected match")
	}
}

func TestTrigger_MatchesEvent_WrongState(t *testing.T) {
	tr := &Trigger{
		Event: "state_change",
		State: "idle",
	}
	evt := monitor.AgentEvent{
		Type:      monitor.EventStateChange,
		Timestamp: time.Now(),
		Data:      monitor.StateChangeData{State: monitor.StateActive},
	}
	if tr.MatchesEvent(evt) {
		t.Fatal("expected no match")
	}
}

func TestTrigger_MatchesEvent_WildcardState(t *testing.T) {
	tr := &Trigger{
		Event: "state_change",
	}
	evt := monitor.AgentEvent{
		Type:      monitor.EventStateChange,
		Timestamp: time.Now(),
		Data:      monitor.StateChangeData{State: monitor.StateActive},
	}
	if !tr.MatchesEvent(evt) {
		t.Fatal("expected match with wildcard state")
	}
}

func TestTrigger_MatchesEvent_SubState(t *testing.T) {
	tr := &Trigger{
		Event:    "state_change",
		SubState: "usage_limit",
	}
	evt := monitor.AgentEvent{
		Type:      monitor.EventStateChange,
		Timestamp: time.Now(),
		Data:      monitor.StateChangeData{State: monitor.StateActive, SubState: monitor.SubStateUsageLimit},
	}
	if !tr.MatchesEvent(evt) {
		t.Fatal("expected match on substate")
	}
}

func TestTrigger_MatchesEvent_SubStateMismatch(t *testing.T) {
	tr := &Trigger{
		Event:    "state_change",
		SubState: "usage_limit",
	}
	evt := monitor.AgentEvent{
		Type:      monitor.EventStateChange,
		Timestamp: time.Now(),
		Data:      monitor.StateChangeData{State: monitor.StateActive, SubState: monitor.SubStateThinking},
	}
	if tr.MatchesEvent(evt) {
		t.Fatal("expected no match on substate mismatch")
	}
}

func TestTrigger_MatchesEvent_WrongEventType(t *testing.T) {
	tr := &Trigger{
		Event: "state_change",
	}
	evt := monitor.AgentEvent{
		Type:      monitor.EventApprovalRequested,
		Timestamp: time.Now(),
	}
	if tr.MatchesEvent(evt) {
		t.Fatal("expected no match on wrong event type")
	}
}

func TestTrigger_MatchesEvent_ApprovalRequested(t *testing.T) {
	tr := &Trigger{
		Event: "approval_requested",
	}
	evt := monitor.AgentEvent{
		Type:      monitor.EventApprovalRequested,
		Timestamp: time.Now(),
		Data:      &monitor.ApprovalRequestedData{ToolName: "Bash"},
	}
	if !tr.MatchesEvent(evt) {
		t.Fatal("expected match on approval_requested")
	}
}

func TestTrigger_MatchesEvent_SessionRotated(t *testing.T) {
	tr := &Trigger{Event: "session_rotated"}
	evt := monitor.AgentEvent{
		Type:      monitor.EventSessionRotated,
		Timestamp: time.Now(),
		Data:      monitor.SessionRotatedData{OldProfile: "default", NewProfile: "alt1"},
	}
	if !tr.MatchesEvent(evt) {
		t.Fatal("expected match on session_rotated")
	}
	// Should not match session_restarted.
	evt2 := monitor.AgentEvent{
		Type:      monitor.EventSessionRestarted,
		Timestamp: time.Now(),
		Data:      monitor.SessionRestartedData{},
	}
	if tr.MatchesEvent(evt2) {
		t.Fatal("session_rotated trigger should not match session_restarted event")
	}
}

func TestTrigger_MatchesEvent_SessionRestarted(t *testing.T) {
	tr := &Trigger{Event: "session_restarted"}
	evt := monitor.AgentEvent{
		Type:      monitor.EventSessionRestarted,
		Timestamp: time.Now(),
		Data:      monitor.SessionRestartedData{},
	}
	if !tr.MatchesEvent(evt) {
		t.Fatal("expected match on session_restarted")
	}
}

func TestEvalCondition_Empty(t *testing.T) {
	if !EvalCondition(t.Context(), "", nil) {
		t.Fatal("empty condition should return true")
	}
}

func TestEvalCondition_TrueCmd(t *testing.T) {
	if !EvalCondition(t.Context(), "true", nil) {
		t.Fatal("'true' should return true")
	}
}

func TestEvalCondition_FalseCmd(t *testing.T) {
	if EvalCondition(t.Context(), "false", nil) {
		t.Fatal("'false' should return false")
	}
}

func TestEvalCondition_EnvVar(t *testing.T) {
	env := map[string]string{"H2_AGENT_STATE": "idle"}
	if !EvalCondition(t.Context(), `test "$H2_AGENT_STATE" = "idle"`, env) {
		t.Fatal("expected condition to pass with env var")
	}
	if EvalCondition(t.Context(), `test "$H2_AGENT_STATE" = "active"`, env) {
		t.Fatal("expected condition to fail with wrong env var value")
	}
}
