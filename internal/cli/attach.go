package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"xanax/internal/attach"
)

func newAttachCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "attach <session-id>",
		Short: "Attach to a running session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			e, err := loadEnv()
			if err != nil {
				return err
			}
			st, err := e.openStore()
			if err != nil {
				return err
			}
			defer st.Close()

			sess, err := st.GetSession(args[0])
			if err != nil {
				return err
			}
			if !attach.Alive(e.socketPath(sess.ID)) {
				if sess.Status.Terminal() {
					return fmt.Errorf("session %s has ended (%s); use `xanax resume %s`",
						shortID(sess.ID), sess.Status, shortID(sess.ID))
				}
				return fmt.Errorf("session %s is not reachable; use `xanax resume %s`",
					shortID(sess.ID), shortID(sess.ID))
			}
			return runAttach(e, sess.ID)
		},
	}
}
