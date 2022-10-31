//go:build !linux

package command

import (
	"errors"

	"github.com/spf13/cobra"
)

const (
	loDevice       = "lo"
	tunDevice      = "tun0"
	tunNetworkAddr = "10.1.1.1/24"
)

func newRunContainerCmd() *runContainerCmd {
	c := &runContainerCmd{}

	cmd := &cobra.Command{
		Use:    "runc [flags] command",
		Short:  "Internal command to run a new process inside an isolated container",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			return errors.New("this command is not supported by your OS")
		},
	}

	c.cmd = cmd
	return c
}

type runContainerCmd struct {
	cmd *cobra.Command
}
