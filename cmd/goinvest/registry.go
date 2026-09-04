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

			report, err := app.Registry(ctx, app.RegistryConfig{
				DB:         deps.DB,
				Identity:   deps.B3,
				Force:      force,
				Now:        time.Now,
				OnProgress: progressWriter(out, interactive),
			})
			if err != nil {
				return err
			}

			// A última linha de progresso ficou sem quebra para poder ser
			// reescrita: sem isto o resumo sairia grudado nela.
			if interactive {
				fmt.Fprintln(out)
			}
			fmt.Fprintln(out, registrySummary(report))
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "ignora o cache e rebate na fonte")
	return cmd
}

// Em terminal a mesma linha é reescrita; num log, cada atualização é uma linha
// nova, senão o registro da execução fica sem histórico.
func progressWriter(out io.Writer, interactive bool) func(registry.Progress) {
	format := "cadastro: %d/%d\n"
	if interactive {
		format = "\rcadastro: %d/%d"
	}
	return func(p registry.Progress) {
		fmt.Fprintf(out, format, p.Done, p.Total)
	}
}

func registrySummary(r registry.Report) string {
	line := fmt.Sprintf("✓ cadastro · %d de %d ativos casados", r.Matched, r.Total)
	if r.Unmatched > 0 {
		line += fmt.Sprintf(" · %d sem correspondência na B3", r.Unmatched)
	}
	if r.Cancelled {
		return fmt.Sprintf("⚠ cadastro interrompido · %d de %d ativos casados · rode de novo para continuar",
			r.Matched, r.Total)
	}
	return line
}
