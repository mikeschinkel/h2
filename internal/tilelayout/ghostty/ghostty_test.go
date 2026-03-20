package ghostty

import (
	"strings"
	"testing"

	"h2/internal/tilelayout"
)

func TestGenerateTabScript_TwoAgents(t *testing.T) {
	tab, _ := tilelayout.ComputeTabLayout([]string{"a1", "a2"}, tilelayout.ScreenSize{Cols: 240, Rows: 60}, 0, tilelayout.DefaultConfig())
	script := generateTabScript(tab, true)

	// Cols-first: 2 agents → 2 columns, 1 right split, no down splits.
	if !strings.Contains(script, "direction right") {
		t.Error("expected split right")
	}
	if strings.Contains(script, "direction down") {
		t.Error("unexpected split down for 2 agents in 2 columns")
	}

	// Should type attach for a2 but not a1 (a1 gets exec'd).
	if !strings.Contains(script, `h2 attach a2`) {
		t.Error("missing attach command for a2")
	}
	if strings.Contains(script, `input text "h2 attach a1`) {
		t.Error("a1 should not be typed (it gets exec'd)")
	}
}

func TestGenerateTabScript_ThreeByThree(t *testing.T) {
	agents := []string{"a1", "a2", "a3", "a4", "a5", "a6", "a7", "a8", "a9"}
	tab, remaining := tilelayout.ComputeTabLayout(agents, tilelayout.ScreenSize{Cols: 240, Rows: 60}, 0, tilelayout.DefaultConfig())
	if len(remaining) != 0 {
		t.Errorf("expected no overflow, got %d", len(remaining))
	}
	script := generateTabScript(tab, true)

	// 2 right splits for 3 columns.
	if strings.Count(script, "direction right") != 2 {
		t.Errorf("expected 2 split right, got %d", strings.Count(script, "direction right"))
	}

	// 3 rows per column → 2 down splits x 3 columns = 6.
	if strings.Count(script, "direction down") != 6 {
		t.Errorf("expected 6 split down, got %d", strings.Count(script, "direction down"))
	}

	// a1 should not be typed (gets exec'd), all others should.
	if strings.Contains(script, `input text "h2 attach a1`) {
		t.Error("a1 should not be typed")
	}
	for _, name := range agents[1:] {
		if !strings.Contains(script, `h2 attach `+name) {
			t.Errorf("missing attach command for %s", name)
		}
	}
}

func TestGenerateTabScript_UnevenColumns(t *testing.T) {
	agents := []string{"a1", "a2", "a3", "a4", "a5", "a6", "a7"}
	tab, _ := tilelayout.ComputeTabLayout(agents, tilelayout.ScreenSize{Cols: 240, Rows: 60}, 0, tilelayout.DefaultConfig())
	script := generateTabScript(tab, true)

	// 3 cols: 2 right splits.
	if strings.Count(script, "direction right") != 2 {
		t.Errorf("expected 2 split right, got %d", strings.Count(script, "direction right"))
	}
	// 7 agents in 3x3 grid: cols 0,1 have 3 rows (2 down each), col 2 has 1 (0 down). Total = 4.
	if strings.Count(script, "direction down") != 4 {
		t.Errorf("expected 4 split down, got %d", strings.Count(script, "direction down"))
	}
}

func TestGenerateTabScript_Overflow(t *testing.T) {
	agents := make([]string, 12)
	for i := range agents {
		agents[i] = "a" + string(rune('A'+i))
	}
	tab, remaining := tilelayout.ComputeTabLayout(agents, tilelayout.ScreenSize{Cols: 240, Rows: 60}, 0, tilelayout.DefaultConfig())
	if len(tab.Panes) != 9 {
		t.Errorf("expected 9 panes in tab 0, got %d", len(tab.Panes))
	}
	if len(remaining) != 3 {
		t.Errorf("expected 3 overflow agents, got %d", len(remaining))
	}
}

func TestGenerateTabScript_SinglePane(t *testing.T) {
	tab, _ := tilelayout.ComputeTabLayout([]string{"solo"}, tilelayout.ScreenSize{Cols: 240, Rows: 60}, 0, tilelayout.DefaultConfig())
	script := generateTabScript(tab, true)

	if strings.Contains(script, "split") {
		t.Error("single pane should not have splits")
	}
}

func TestGenerateTabScript_NonFirstTab(t *testing.T) {
	// Non-first tab should type attach for ALL panes including (0,0).
	tab, _ := tilelayout.ComputeTabLayout([]string{"a1", "a2"}, tilelayout.ScreenSize{Cols: 240, Rows: 60}, 1, tilelayout.DefaultConfig())
	script := generateTabScript(tab, false)

	if !strings.Contains(script, `h2 attach a1`) {
		t.Error("non-first tab should type attach for a1")
	}
	if !strings.Contains(script, `h2 attach a2`) {
		t.Error("non-first tab should type attach for a2")
	}
}
