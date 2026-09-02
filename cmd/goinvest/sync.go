package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/marlliton/goinvest/internal/app"
	"github.com/marlliton/goinvest/internal/collect"
	"github.com/spf13/cobra"
)

func newSyncCmd(deps rootDeps) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Baixa e grava os indicadores de todas as ações e FIIs",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			report, err := app.Sync(cmd.Context(), app.SyncConfig{
				Providers: deps.Providers,
				DB:        deps.DB,
				Force:     force,
				Now:       time.Now,
			})
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintln(out, stageLine("ações", report.Stocks))
			fmt.Fprintln(out, stageLine("FIIs", report.FIIs))
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "ignora o cache e rebate na fonte")
	return cmd
}

func stageLine(label string, r collect.SourceResult) string {
	if r.Status == collect.StatusOK {
		return fmt.Sprintf("✓ %s · %d ativos · %s", label, r.AssetCount, formatSeconds(r.Duration))
	}
	return fmt.Sprintf("✗ %s · %s — dados anteriores preservados", label, r.Reason)
}

func formatSeconds(d time.Duration) string {
	return strings.Replace(fmt.Sprintf("%.1fs", d.Seconds()), ".", ",", 1)
}
