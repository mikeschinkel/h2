// Package ghostty implements tiled pane layout for the Ghostty terminal.
//
// It uses the `ghostty +action` CLI for all operations: splits, navigation,
// tabs, and writing text to panes (via the text: action). No macOS
// Accessibility permissions are required.
//
// Ghostty action names used (verify with `ghostty +list-actions`):
//
//	new_split:right, new_split:down
//	goto_split:up, goto_split:down, goto_split:left, goto_split:right
//	new_tab, previous_tab
//	text:<string>  (writes to focused pane's PTY)
package ghostty

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"h2/internal/tilelayout"
)

// Driver implements tiled agent attachment for Ghostty.
type Driver struct{}

// NewDriver creates a Ghostty tiling driver.
func NewDriver() *Driver { return &Driver{} }

// Script returns the bash script that would create the tiled layout
// without executing it. Used for dry-run output.
func (d *Driver) Script(layout tilelayout.TileLayout) string {
	return generateScript(layout)
}

// DetectFullWindowSize opens a temporary Ghostty tab, types a command
// that writes the terminal size to a temp file, reads it back, then
// closes the tab. This gives the full window size even when invoked
// from within an existing split.
func (d *Driver) DetectFullWindowSize() (tilelayout.ScreenSize, error) {
	if _, err := exec.LookPath("ghostty"); err != nil {
		return tilelayout.ScreenSize{}, fmt.Errorf("ghostty CLI not found")
	}

	// Create a temp file for the new tab's shell to write its size into.
	tmpFile, err := os.CreateTemp("", "h2-winsize-*.txt")
	if err != nil {
		return tilelayout.ScreenSize{}, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	// Open a temp tab.
	if err := exec.Command("ghostty", "+action", "new_tab").Run(); err != nil {
		return tilelayout.ScreenSize{}, fmt.Errorf("open temp tab: %w", err)
	}
	time.Sleep(500 * time.Millisecond)

	// Type a command into the new tab that writes cols/rows to the temp file,
	// then immediately closes the tab via exit.
	sizeCmd := fmt.Sprintf(`echo "$(tput cols) $(tput lines)" > %s; exit`, tmpPath)
	escaped := strings.ReplaceAll(sizeCmd, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	action := fmt.Sprintf("text:%s\\x0a", escaped)
	exec.Command("ghostty", "+action", action).Run()

	// Wait for the command to execute and the tab to close.
	var size tilelayout.ScreenSize
	for i := 0; i < 20; i++ {
		time.Sleep(200 * time.Millisecond)
		data, err := os.ReadFile(tmpPath)
		if err != nil || len(strings.TrimSpace(string(data))) == 0 {
			continue
		}
		parts := strings.Fields(strings.TrimSpace(string(data)))
		if len(parts) == 2 {
			cols, errC := strconv.Atoi(parts[0])
			rows, errR := strconv.Atoi(parts[1])
			if errC == nil && errR == nil && cols > 0 && rows > 0 {
				size = tilelayout.ScreenSize{Cols: cols, Rows: rows}
				break
			}
		}
	}

	// Navigate back to the original tab (exit should have closed the temp tab,
	// but previous_tab is a safe no-op if it already closed).
	exec.Command("ghostty", "+action", "previous_tab").Run()
	time.Sleep(100 * time.Millisecond)

	if size.Cols == 0 {
		return tilelayout.ScreenSize{}, fmt.Errorf("failed to read window size from temp tab")
	}
	return size, nil
}

// Tile creates Ghostty splits, types `h2 attach` in each pane, then
// execs into the first agent's attach session in the current pane.
func (d *Driver) Tile(layout tilelayout.TileLayout, h2Binary string) error {
	if len(layout.Tabs) == 0 || len(layout.Tabs[0].Panes) == 0 {
		return fmt.Errorf("empty layout")
	}

	firstAgent := layout.Tabs[0].Panes[0].AgentName

	// Single pane: no splits needed, just exec directly.
	if layout.TotalPanes() == 1 {
		return syscall.Exec(h2Binary, []string{"h2", "attach", firstAgent}, os.Environ())
	}

	// Verify ghostty CLI is available.
	if _, err := exec.LookPath("ghostty"); err != nil {
		return fmt.Errorf("ghostty CLI not found in PATH; ensure Ghostty CLI integration is enabled")
	}

	script := generateScript(layout)

	// Write and execute setup script.
	f, err := os.CreateTemp("", "h2-tile-*.sh")
	if err != nil {
		return fmt.Errorf("create tile script: %w", err)
	}
	tmpPath := f.Name()
	defer os.Remove(tmpPath)

	if _, err := f.WriteString(script); err != nil {
		f.Close()
		return fmt.Errorf("write tile script: %w", err)
	}
	f.Close()

	if err := os.Chmod(tmpPath, 0o755); err != nil {
		return fmt.Errorf("chmod tile script: %w", err)
	}

	cmd := exec.Command("bash", tmpPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tile setup failed: %w", err)
	}

	// Replace current process with h2 attach for the first agent.
	return syscall.Exec(h2Binary, []string{"h2", "attach", firstAgent}, os.Environ())
}

// generateScript produces a bash script that creates the tiled layout.
//
// Grid build strategy (per tab):
//  1. Create columns by splitting right (C-1 times). Focus ends on rightmost.
//  2. Build rows right-to-left: in each column, split down (R-1 times),
//     then goto_split:left to the next column.
//  3. Navigate to top-left pane.
//  4. Walk column-major through the grid, typing `h2 attach <name>` in
//     each pane (except (0,0) of the first tab, which gets exec'd later).
//
// Note: binary splitting means columns won't be perfectly even for non-power-
// of-2 counts. This is a Ghostty limitation; use equalize if available.
func generateScript(layout tilelayout.TileLayout) string {
	var b strings.Builder
	b.WriteString("#!/bin/bash\n")
	b.WriteString("# Generated by h2 attach --tile\n\n")

	for tabIdx, tab := range layout.Tabs {
		if tabIdx > 0 {
			writeAction(&b, "new_tab")
			writeSleep(&b, 500)
		}

		writeBuildPhase(&b, tab)

		// Wait for shells in new panes to initialize.
		writeSleep(&b, 800)

		writeTypePhase(&b, tab, tabIdx == 0)
	}

	// Return to the first tab if overflow tabs were created.
	if len(layout.Tabs) > 1 {
		b.WriteString("\n# Return to first tab\n")
		for i := 1; i < len(layout.Tabs); i++ {
			writeAction(&b, "previous_tab")
			writeSleep(&b, 100)
		}
	}

	// Navigate to (0,0) in the first tab for the exec.
	// Extra up/left from the boundary are no-ops.
	if len(layout.Tabs) > 0 {
		tab := layout.Tabs[0]
		b.WriteString("\n# Focus top-left pane for exec\n")
		for r := 1; r < tab.Rows; r++ {
			writeAction(&b, "goto_split:up")
		}
		if tab.Rows > 1 {
			writeSleep(&b, 50)
		}
		for c := 1; c < tab.Cols; c++ {
			writeAction(&b, "goto_split:left")
		}
		if tab.Cols > 1 {
			writeSleep(&b, 50)
		}
	}

	return b.String()
}

// writeBuildPhase emits commands to create the split grid for one tab.
func writeBuildPhase(b *strings.Builder, tab tilelayout.TabLayout) {
	if tab.Cols <= 1 && tab.RowsInCol(0) <= 1 {
		return
	}

	b.WriteString("# Build grid structure\n")

	// Create columns by splitting right.
	for c := 1; c < tab.Cols; c++ {
		writeAction(b, "new_split:right")
		writeSleep(b, 300)
	}

	// Build rows in each column, right-to-left.
	// After column splits, focus is on the rightmost column.
	for c := tab.Cols - 1; c >= 0; c-- {
		if c < tab.Cols-1 {
			writeAction(b, "goto_split:left")
			writeSleep(b, 100)
		}
		colRows := tab.RowsInCol(c)
		for r := 1; r < colRows; r++ {
			writeAction(b, "new_split:down")
			writeSleep(b, 300)
		}
	}
	b.WriteByte('\n')
}

// writeTypePhase emits commands to type h2 attach in each pane.
func writeTypePhase(b *strings.Builder, tab tilelayout.TabLayout, isFirstTab bool) {
	if len(tab.Panes) <= 1 && isFirstTab {
		return
	}

	b.WriteString("# Type attach commands\n")

	// After build phase, focus is at bottom of leftmost column.
	// Navigate up to (0,0).
	col0Rows := tab.RowsInCol(0)
	for r := 1; r < col0Rows; r++ {
		writeAction(b, "goto_split:up")
		writeSleep(b, 50)
	}

	for c := 0; c < tab.Cols; c++ {
		if c > 0 {
			// Return to top of previous column, then move right.
			prevColRows := tab.RowsInCol(c - 1)
			for r := 1; r < prevColRows; r++ {
				writeAction(b, "goto_split:up")
				writeSleep(b, 50)
			}
			writeAction(b, "goto_split:right")
			writeSleep(b, 50)
		}

		colRows := tab.RowsInCol(c)
		for r := 0; r < colRows; r++ {
			if isFirstTab && c == 0 && r == 0 {
				// Skip (0,0) of first tab — will exec there later.
				if colRows > 1 {
					writeAction(b, "goto_split:down")
					writeSleep(b, 50)
				}
				continue
			}

			paneIdx := c*tab.Rows + r
			if paneIdx < len(tab.Panes) {
				writeTypeAttach(b, tab.Panes[paneIdx].AgentName)
			}

			if r < colRows-1 {
				writeAction(b, "goto_split:down")
				writeSleep(b, 50)
			}
		}
	}
	b.WriteByte('\n')
}

func writeAction(b *strings.Builder, action string) {
	fmt.Fprintf(b, "ghostty +action %s\n", action)
}

func writeSleep(b *strings.Builder, ms int) {
	fmt.Fprintf(b, "sleep %.2f\n", float64(ms)/1000)
}

func writeTypeAttach(b *strings.Builder, agentName string) {
	// Use Ghostty's text: action to write directly to the focused pane's PTY.
	// No macOS Accessibility permissions required (unlike osascript keystroke).
	// \x0a is a newline (Enter) in Ghostty's text action escape syntax.
	escaped := strings.ReplaceAll(agentName, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	fmt.Fprintf(b, "ghostty +action \"text:h2 attach %s\\x0a\"\n", escaped)
	writeSleep(b, 300)
}
