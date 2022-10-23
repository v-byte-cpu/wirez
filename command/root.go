package command

import (
	"errors"
	"os"
	"os/exec"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

func Main(version string) {
	log := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}).With().Timestamp().Logger()
	if err := newRootCmd(&log, version).Execute(); err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			os.Exit(exitError.ExitCode())
		}
		log.Error().Err(err).Msg("")
		os.Exit(1)
	}
}

func newRootCmd(log *zerolog.Logger, version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "wirez",
		Short:         "socks5 proxy rotator",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.AddCommand(
		newServerCmd(log).cmd,
		newRunCmd(log).cmd,
		newRunContainerCmd().cmd,
	)

	return cmd
}
