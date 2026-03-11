package cmd

import (
	"fmt"
	"net"
	"os"
	"text/tabwriter"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"h2/internal/automation"
	"h2/internal/session/message"
	"h2/internal/socketdir"
)

func newTriggerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trigger",
		Short: "Manage triggers on an agent",
		Long:  "Register, list, and remove event-driven triggers on a running agent.",
	}

	cmd.AddCommand(
		newTriggerAddCmd(),
		newTriggerListCmd(),
		newTriggerRemoveCmd(),
	)
	return cmd
}

func newTriggerAddCmd() *cobra.Command {
	var (
		event      string
		state      string
		subState   string
		condition  string
		execCmd    string
		msg        string
		from       string
		priority   string
		name       string
		maxFirings int
		expiresAt  string
		cooldown   string
	)

	cmd := &cobra.Command{
		Use:   "add <agent-name>",
		Short: "Register a trigger on an agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentName := args[0]

			if execCmd == "" && msg == "" {
				return fmt.Errorf("either --exec or --message is required")
			}
			if execCmd != "" && msg != "" {
				return fmt.Errorf("--exec and --message are mutually exclusive")
			}

			id := uuid.New().String()[:8]

			spec := &message.TriggerSpec{
				ID:         id,
				Name:       name,
				Event:      event,
				State:      state,
				SubState:   subState,
				Condition:  condition,
				Exec:       execCmd,
				Message:    msg,
				From:       from,
				Priority:   priority,
				MaxFirings: maxFirings,
				Cooldown:   cooldown,
			}

			// Handle relative ExpiresAt ("+1h") by resolving to absolute timestamp.
			if expiresAt != "" {
				if len(expiresAt) > 0 && expiresAt[0] == '+' {
					resolved, err := automation.ResolveExpiresAt(expiresAt, time.Now())
					if err != nil {
						return fmt.Errorf("invalid --expires-at: %w", err)
					}
					spec.ExpiresAt = resolved.Format(time.RFC3339)
				} else {
					spec.ExpiresAt = expiresAt
				}
			}

			resp, err := sendSocketRequest(agentName, &message.Request{
				Type:    "trigger_add",
				Trigger: spec,
			})
			if err != nil {
				return err
			}
			if !resp.OK {
				return fmt.Errorf("trigger add failed: %s", resp.Error)
			}

			fmt.Fprintf(os.Stderr, "Note: dynamically registered, will not survive agent restart.\n")
			fmt.Println(resp.TriggerID)
			return nil
		},
	}

	cmd.Flags().StringVar(&event, "event", "", "Event type to match (e.g. state_change, approval_requested)")
	cmd.Flags().StringVar(&state, "state", "", "State to match (for state_change events)")
	cmd.Flags().StringVar(&subState, "sub-state", "", "SubState to match (for state_change events)")
	cmd.Flags().StringVar(&condition, "condition", "", "Shell command condition (exit 0 = pass)")
	cmd.Flags().StringVar(&execCmd, "exec", "", "Shell command action")
	cmd.Flags().StringVar(&msg, "message", "", "Message action (injected into agent PTY)")
	cmd.Flags().StringVar(&from, "from", "", "Sender identity for message action (default: h2-trigger)")
	cmd.Flags().StringVar(&priority, "priority", "", "Message priority (interrupt|normal|idle-first|idle)")
	cmd.Flags().StringVar(&name, "name", "", "Human-readable trigger name")
	cmd.Flags().IntVar(&maxFirings, "max-firings", 0, "Max times to fire (-1=unlimited, default 1=one-shot)")
	cmd.Flags().StringVar(&expiresAt, "expires-at", "", "Expiry timestamp (RFC 3339) or relative (+1h, +30m)")
	cmd.Flags().StringVar(&cooldown, "cooldown", "", "Minimum duration between firings (e.g. 5m, 30s)")
	_ = cmd.MarkFlagRequired("event")

	return cmd
}

func newTriggerListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <agent-name>",
		Short: "List triggers on an agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := sendSocketRequest(args[0], &message.Request{
				Type: "trigger_list",
			})
			if err != nil {
				return err
			}
			if !resp.OK {
				return fmt.Errorf("trigger list failed: %s", resp.Error)
			}

			if len(resp.Triggers) == 0 {
				fmt.Println("No triggers registered.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tEVENT\tSTATE\tSUBSTATE\tACTION\tMAX_FIRINGS\tFIRE_COUNT\tCOOLDOWN")
			for _, t := range resp.Triggers {
				action := "exec"
				if t.Message != "" {
					action = "message"
				}
				maxF := "1" // default one-shot
				if t.MaxFirings == -1 {
					maxF = "-"
				} else if t.MaxFirings > 0 {
					maxF = fmt.Sprintf("%d", t.MaxFirings)
				}
				cd := "-"
				if t.Cooldown != "" {
					cd = t.Cooldown
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%d\t%s\n",
					t.ID, t.Name, t.Event, t.State, t.SubState, action, maxF, t.FireCount, cd)
			}
			w.Flush()
			return nil
		},
	}
}

func newTriggerRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <agent-name> <trigger-id>",
		Short: "Remove a trigger from an agent",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := sendSocketRequest(args[0], &message.Request{
				Type:      "trigger_remove",
				TriggerID: args[1],
			})
			if err != nil {
				return err
			}
			if !resp.OK {
				return fmt.Errorf("trigger remove failed: %s", resp.Error)
			}
			fmt.Println("Removed.")
			return nil
		},
	}
}

// sendSocketRequest connects to an agent's daemon socket and sends a request.
func sendSocketRequest(agentName string, req *message.Request) (*message.Response, error) {
	sockPath, err := socketdir.Find(agentName)
	if err != nil {
		return nil, agentConnError(agentName, err)
	}
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		return nil, agentConnError(agentName, err)
	}
	defer conn.Close()

	if err := message.SendRequest(conn, req); err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	resp, err := message.ReadResponse(conn)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	return resp, nil
}
