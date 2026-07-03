package cli

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Print resolved configuration and paths",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			e, err := loadEnv()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			source := e.paths.ConfigFile
			if _, err := os.Stat(source); os.IsNotExist(err) {
				source += " (not present, using built-in defaults)"
			}
			fmt.Fprintf(out, "# config:  %s\n", source)
			fmt.Fprintf(out, "# data:    %s\n", e.paths.DataDir)
			fmt.Fprintf(out, "# db:      %s\n", e.paths.DBFile)
			fmt.Fprintf(out, "# logs:    %s\n", e.paths.LogsDir)
			fmt.Fprintf(out, "# sockets: %s\n\n", e.paths.SocketDir)
			return toml.NewEncoder(out).Encode(e.cfg)
		},
	}
}
