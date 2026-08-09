package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newHideCmd stashes one or more sessions out of the dashboard list. Hiding is
// a rvr-only view flag; it never touches the harness session or its lifecycle.
func newHideCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "hide <session-id>...",
		Short: "Hide sessions from the dashboard list",
		Args:  cobra.MinimumNArgs(1),
		RunE:  setHidden(false),
	}
}

// newShowCmd restores one or more hidden sessions back into the dashboard list.
func newShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <session-id>...",
		Short: "Restore hidden sessions to the dashboard list",
		Args:  cobra.MinimumNArgs(1),
		RunE:  setHidden(true),
	}
}

// setHidden builds the shared RunE for hide/show: resolve each id (git-style
// prefix) and set the hidden flag. hidden=true restores (un-hides); false hides.
func setHidden(visible bool) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		e, err := loadEnv()
		if err != nil {
			return err
		}
		st, err := e.openStore()
		if err != nil {
			return err
		}
		defer st.Close()

		verb := "hidden"
		if visible {
			verb = "shown"
		}
		var applied []string
		for _, id := range args {
			sess, err := st.GetSession(id)
			if err != nil {
				return err
			}
			if err := st.SetHidden(sess.ID, !visible); err != nil {
				return err
			}
			applied = append(applied, shortID(sess.ID))
		}
		if len(applied) == 1 {
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s.\n", applied[0], verb)
			return nil
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s %d sessions.\n", verb, len(applied))
		return nil
	}
}
