//go:build !linux

package command

import (
	"errors"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

func newRunCmd(log *zerolog.Logger) *runCmd {
	c := &runCmd{}

	cmd := &cobra.Command{
		Use:    "run [flags] command",
		Short:  "Proxy application traffic through the socks5 server",
		Long:   "Run a command in an unprivileged container that transparently proxies application traffic through the socks5 server",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			return errors.New("this command is not supported by your OS")
		},
	}

	c.cmd = cmd
	return c
}

type runCmd struct {
	cmd *cobra.Command
}
