// Package cli wires the xanax command tree (SPEC.md §9).
package cli

import (
	"os"
	"time"

	"github.com/spf13/cobra"

	"xanax/internal/config"
	"xanax/internal/store"
	"xanax/internal/tui"
)

// Execute runs the root command and returns its error for main to report.
func Execute() error {
	return newRootCmd().Execute()
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "xanax",
		Short: "Session manager for autonomous AI coding agents",
		Long: `Xanax launches, supervises, and reattaches to autonomous AI coding agent
sessions (opencode, pi, ...) so they keep running when your terminal doesn't.`,
		Version:       "0.1.0-dev",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDashboard()
		},
	}
	root.AddCommand(
		newNewCmd(),
		newListCmd(),
		newAttachCmd(),
		newResumeCmd(),
		newKillCmd(),
		newConfigCmd(),
		newSuperviseCmd(),
	)
	return root
}

// env bundles the resolved paths and configuration every command needs.
type env struct {
	paths config.Paths
	cfg   *config.Config
}

func loadEnv() (*env, error) {
	paths, err := config.DefaultPaths()
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(paths.ConfigFile)
	if err != nil {
		return nil, err
	}
	return &env{paths: paths, cfg: cfg}, nil
}

func (e *env) openStore() (*store.Store, error) {
	return store.Open(e.paths.DBFile)
}

// runDashboard reconciles interrupted sessions (auto-resume) and launches the
// dashboard TUI.
func runDashboard() error {
	e, err := loadEnv()
	if err != nil {
		return err
	}
	st, err := e.openStore()
	if err != nil {
		return err
	}
	defer st.Close()

	if resumed, err := e.reconcile(st); err == nil && len(resumed) > 0 {
		// Give freshly auto-resumed supervisors a moment to bind their sockets
		// before the first render so they show as running, not interrupted.
		e.waitForSocket(resumed[0], 3*time.Second)
	}

	self, err := os.Executable()
	if err != nil {
		return err
	}
	return tui.Run(tui.Deps{
		Store:     st,
		Cfg:       e.cfg,
		SelfPath:  self,
		SocketDir: e.paths.SocketDir,
	})
}
