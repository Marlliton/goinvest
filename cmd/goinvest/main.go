package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	os.Exit(run())
}

func run() int {
	deps, err := build()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer deps.DB.Close()

	rootCmd := &cobra.Command{
		Use:           "goinvest",
		Short:         "Análise fundamentalista de ativos brasileiros no terminal",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	rootCmd.AddCommand(newSyncCmd(deps), newShowCmd(deps), newRegistryCmd(deps))

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
