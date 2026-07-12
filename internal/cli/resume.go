package cli

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/LeJamon/rvr/internal/attach"
	"github.com/LeJamon/rvr/internal/session"
	"github.com/LeJamon/rvr/internal/store"
)

func newResumeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "resume <session-id>",
		Short: "Reattach to a live session, or relaunch a dead one via the harness's native resume",
		Long: `Reattach to a live session, or relaunch a dead one via the harness's native resume.

Inside the session window, press Left arrow or ctrl+\ to detach. The session
keeps running after you detach.`,
		Args: cobra.ExactArgs(1),
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
			out := cmd.OutOrStdout()

			// Already running: just reattach.
			if attach.Alive(e.socketPath(sess.ID)) {
				return runAttach(e, sess.ID)
			}

			// Dead: relaunch via the harness's native resume.
			h, ok := e.cfg.Harnesses[sess.Harness]
			if !ok {
				return fmt.Errorf("session %s uses unknown harness %q", shortID(sess.ID), sess.Harness)
			}
			if !e.canResume(sess) {
				return fmt.Errorf("session %s cannot be resumed (no harness session ref, and its harness has no resume_args)",
					shortID(sess.ID))
			}
			if err := e.checkHarnessCommand(sess.Harness, h); err != nil {
				return err
			}
			if err := st.BeginResume(sess); errors.Is(err, store.ErrConflict) {
				wait := 10 * time.Second
				if alive, terminal := e.waitForSocketOrTerminal(st, sess.ID, wait); alive {
					return runAttach(e, sess.ID)
				} else if terminal != nil {
					return e.sessionUnavailableError(st, terminal)
				}
				return e.supervisorStartingError(sess.ID, wait)
			} else if err != nil {
				return fmt.Errorf("begin resume for session %s: %w", shortID(sess.ID), err)
			}
			if _, err := e.spawnSupervisor(sess.ID, true); err != nil {
				detail := "resume supervisor failed: " + err.Error()
				_ = st.FinishWithDetail(sess.ID, session.StatusFailed, 1, detail)
				return err
			}
			st.RecordEvent(sess.ID, "resumed", map[string]any{"auto": false})
			fmt.Fprintf(out, "Resuming session %s (%s)...\n", shortID(sess.ID), sess.Harness)
			wait := 10 * time.Second
			if alive, terminal := e.waitForSocketOrTerminal(st, sess.ID, wait); !alive {
				if terminal != nil {
					return e.sessionUnavailableError(st, terminal)
				}
				return e.supervisorStartingError(sess.ID, wait)
			}
			return runAttach(e, sess.ID)
		},
	}
}
