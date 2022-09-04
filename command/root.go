package command

import (
	"os"

	"github.com/spf13/cobra"
)

func Main(version string) {
	if err := newRootCmd(version).Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "wirez",
		Short:   "socks5 proxy rotator",
		Version: version,
	}

	cmd.AddCommand(
		newServerCmd().cmd,
	)

	return cmd
}
