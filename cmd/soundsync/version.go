package main

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the Soundsync version",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "soundsync %s (%s, %s/%s)\n",
				version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
		},
	}
}
