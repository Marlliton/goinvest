package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"time"

	"github.com/marlliton/goinvest/internal/app"
	"github.com/marlliton/goinvest/internal/collect"
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

			_, cancelled, err := runStage(ctx, out, interactive, "ações", func() (registry.Report, error) {
				return app.Registry(ctx, app.RegistryConfig{
					DB:         deps.DB,
					Identity:   deps.B3,
					Force:      force,
					Now:        time.Now,
					OnProgress: progressWriter(out, interactive, "ações"),
				})
			})
			if err != nil {
				return err
			}

			if cancelled {
				fmt.Fprintln(out, "⚠ cadastro de FIIs pulado: coleta de ações foi interrompida")
			} else {
				_, cancelled, err = runStage(ctx, out, interactive, "FIIs", func() (registry.Report, error) {
					return app.RegistryFII(ctx, app.RegistryFIIConfig{
						DB:          deps.DB,
						CVM:         deps.CVM,
						Fundamentus: deps.Fundamentus,
						Force:       force,
						Now:         time.Now,
						OnProgress:  progressWriter(out, interactive, "FIIs"),
					})
				})
				if err != nil {
					return err
				}
			}

			if deps.Catalog != nil {
				if err := deps.DB.RecomputeSectorStats(context.WithoutCancel(ctx), collect.MetricRules(deps.Catalog), time.Now()); err != nil {
					return fmt.Errorf("recalcular referência setorial: %w", err)
				}
			}

			if cancelled {
				return errors.New("cadastro interrompido pelo usuário")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "ignora o cache e rebate na fonte")
	return cmd
}

// runStage isola a detecção de cancelamento do erro cru devolvido pela fonte:
// um Ctrl-C durante a coleta propaga context.Canceled envolto na URL da
// requisição em andamento, e esse texto nunca deve chegar ao usuário.
func runStage(ctx context.Context, out io.Writer, interactive bool, label string, run func() (registry.Report, error)) (report registry.Report, cancelled bool, err error) {
	report, runErr := run()
	endProgress(out, interactive)

	if runErr != nil {
		if ctx.Err() != nil {
			fmt.Fprintf(out, "⚠ cadastro de %s interrompido pelo usuário\n", label)
			return registry.Report{}, true, nil
		}
		return registry.Report{}, false, runErr
	}

	fmt.Fprintln(out, registrySummary(label, report))
	return report, report.Cancelled, nil
}

func progressWriter(out io.Writer, interactive bool, label string) func(registry.Progress) {
	format := "cadastro: %s %d/%d\n"
	if interactive {
		format = "\rcadastro: %s %d/%d"
	}
	return func(p registry.Progress) {
		fmt.Fprintf(out, format, label, p.Done, p.Total)
	}
}

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
