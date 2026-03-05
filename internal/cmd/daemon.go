package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"h2/internal/config"
	"h2/internal/session"
)

func newDaemonCmd() *cobra.Command {
	var sessionDir string
	var resume bool

	cmd := &cobra.Command{
		Use:    "_daemon --session-dir=<path>",
		Short:  "Run as a daemon (internal)",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if sessionDir == "" {
				return fmt.Errorf("--session-dir is required")
			}

			rc, err := config.ReadRuntimeConfig(sessionDir)
			if err != nil {
				return fmt.Errorf("read runtime config: %w", err)
			}

			err = session.RunDaemon(sessionDir, rc, resume)
			if err != nil {
				if _, ok := err.(*exec.ExitError); ok {
					os.Exit(1)
				}
				return err
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&sessionDir, "session-dir", "", "Session directory path containing session.metadata.json")
	cmd.Flags().BoolVar(&resume, "resume", false, "Resume the harness session instead of starting fresh")

	return cmd
}
