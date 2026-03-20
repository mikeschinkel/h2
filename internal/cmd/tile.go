package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"golang.org/x/term"

	"h2/internal/socketdir"
	"h2/internal/tilelayout"
	"h2/internal/tilelayout/ghostty"
)

var tileAttachFunc = doTileAttach

func terminalSupportsTileAttach() bool {
	termProgram := strings.ToLower(os.Getenv("TERM_PROGRAM"))
	termName := strings.ToLower(os.Getenv("TERM"))
	return termProgram == "ghostty" || strings.Contains(termName, "ghostty")
}

// doTileAttach resolves agents from name, computes a tiled layout, and
// opens Ghostty splits with h2 attach in each pane.
func doTileAttach(name string, dryRun bool) error {
	agents, err := resolveTileAgents(name)
	if err != nil {
		return err
	}
	if len(agents) == 0 {
		return fmt.Errorf("no running agents found for %q", name)
	}

	// Single agent without dry-run: just do a normal attach.
	if len(agents) == 1 && !dryRun {
		return doAttach(agents[0])
	}

	fd := int(os.Stdin.Fd())
	cols, rows, err := term.GetSize(fd)
	if err != nil {
		return fmt.Errorf("get terminal size: %w", err)
	}

	currentSize := tilelayout.ScreenSize{Cols: cols, Rows: rows}
	cfg := tilelayout.DefaultConfig()

	// Compute only the first tab with the current pane size.
	tab0, overflow := tilelayout.ComputeTabLayout(agents, currentSize, 0, cfg)
	layout := tilelayout.TileLayout{Tabs: []tilelayout.TabLayout{tab0}}

	if dryRun {
		tilelayout.PrintDryRun(layout, overflow, os.Stdout)
		driver := ghostty.NewDriver()
		fmt.Println("\nScript for tab 1:")
		fmt.Println(driver.ScriptForTab(tab0, true))
		return nil
	}

	h2Binary, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve h2 binary path: %w", err)
	}

	driver := ghostty.NewDriver()
	return driver.TileIterative(tab0, overflow, cfg, h2Binary)
}

// resolveTileAgents interprets name as:
//  1. A comma-separated list of agent names (if commas present).
//  2. A running pod name (checked first for single names; pod wins on collision).
//  3. A single agent name.
func resolveTileAgents(name string) ([]string, error) {
	parts := strings.Split(name, ",")
	if len(parts) > 1 {
		var agents []string
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				agents = append(agents, p)
			}
		}
		if len(agents) == 0 {
			return nil, fmt.Errorf("no agent names provided")
		}
		return agents, nil
	}

	// Single name: check if it's a running pod.
	name = strings.TrimSpace(name)
	if agents := podAgentNamesSorted(name); len(agents) > 0 {
		return agents, nil
	}

	// Not a pod — treat as single agent name.
	return []string{name}, nil
}

// podAgentNamesSorted returns sorted agent names for the given running pod.
func podAgentNamesSorted(podName string) []string {
	entries, err := socketdir.ListByType(socketdir.TypeAgent)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		info := queryAgent(e.Path)
		if info != nil && info.Pod == podName {
			names = append(names, info.Name)
		}
	}
	sort.Strings(names)
	return names
}
