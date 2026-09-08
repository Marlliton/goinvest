package main

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"time"

	"github.com/marlliton/goinvest/internal/app"
	"github.com/marlliton/goinvest/internal/registry"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

func newRegistryCmd(deps rootDeps) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "registry",
		Short: "Sincroniza identidade e setor oficial das ações (B3)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
			defer stop()

			out := cmd.OutOrStdout()
			interactive := isatty.IsTerminal(os.Stdout.Fd())

			result, err := app.RegistryAll(ctx, app.RegistryAllConfig{
				Stocks: app.RegistryConfig{
					DB:         deps.DB,
					Identity:   deps.B3,
					Force:      force,
					Now:        time.Now,
					OnProgress: progressWriter(out, interactive, "ações"),
				},
				FIIs: app.RegistryFIIConfig{
					DB:          deps.DB,
					CVM:         deps.CVM,
					Fundamentus: deps.Fundamentus,
					Force:       force,
					Now:         time.Now,
					OnProgress:  progressWriter(out, interactive, "FIIs"),
				},
				Catalog: deps.Catalog,
				Now:     time.Now,
			})
			endProgress(out, interactive)

			printStageOutcome(out, "ações", result.Stocks, result.StocksCancelled)
			if result.FIIsSkipped {
				fmt.Fprintln(out, "⚠ cadastro de FIIs pulado: coleta de ações foi interrompida")
			} else {
				printStageOutcome(out, "FIIs", result.FIIs, result.FIIsCancelled)
			}

			return err
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "ignora o cache e rebate na fonte")
	return cmd
}

// O cancelamento chega de dois jeitos: como erro cru embrulhado (sem relatório)
// ou já limpo dentro de registry.Report.
func printStageOutcome(out io.Writer, label string, report registry.Report, cancelled bool) {
	if cancelled && !report.Cancelled {
		fmt.Fprintf(out, "⚠ cadastro de %s interrompido pelo usuário\n", label)
		return
	}
	fmt.Fprintln(out, registrySummary(label, report))
}

func registrySummary(label string, r registry.Report) string {
	if r.Cancelled {
		return fmt.Sprintf("⚠ cadastro de %s interrompido · %d de %d casados · rode de novo para continuar",
			label, r.Matched, r.Total)
	}
	line := fmt.Sprintf("✓ cadastro de %s · %d de %d casados", label, r.Matched, r.Total)
	if r.Unmatched > 0 {
		line += fmt.Sprintf(" · %d sem correspondência", r.Unmatched)
	}
	return line
}
