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

			stocks, err := app.Registry(ctx, app.RegistryConfig{
				DB:         deps.DB,
				Identity:   deps.B3,
				Force:      force,
				Now:        time.Now,
				OnProgress: progressWriter(out, interactive, "ações"),
			})
			if err != nil {
				return err
			}
			endProgress(out, interactive)
			fmt.Fprintln(out, registrySummary("ações", stocks))

			fiis, err := app.RegistryFII(ctx, app.RegistryFIIConfig{
				DB:          deps.DB,
				CVM:         deps.CVM,
				Fundamentus: deps.Fundamentus,
				Force:       force,
				Now:         time.Now,
				OnProgress:  progressWriter(out, interactive, "FIIs"),
			})
			if err != nil {
				return err
			}
			endProgress(out, interactive)
			fmt.Fprintln(out, registrySummary("FIIs", fiis))
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "ignora o cache e rebate na fonte")
	return cmd
}

// Em terminal a mesma linha é reescrita; num log, cada atualização é uma linha
// nova, senão o registro da execução fica sem histórico.
func progressWriter(out io.Writer, interactive bool, label string) func(registry.Progress) {
	format := "cadastro: %s %d/%d\n"
	if interactive {
		format = "\rcadastro: %s %d/%d"
	}
	return func(p registry.Progress) {
		fmt.Fprintf(out, format, label, p.Done, p.Total)
	}
}

// A linha de progresso interativa fica sem quebra para poder ser reescrita:
// sem isto o resumo sairia grudado nela.
func endProgress(out io.Writer, interactive bool) {
	if interactive {
		fmt.Fprintln(out)
	}
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
