package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/marlliton/goinvest/internal/collect"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

// Mesmo teto do compare: detalhar é o passo que prepara a comparação, e uma
// lista maior que a comparável só gastaria requisição.
const maxDeepTickers = 8

func newDetalharCmd(deps rootDeps) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "detalhar TICKER...",
		Short: "Coleta detalhe e proventos dos tickers informados",
		Args:  cobra.RangeArgs(1, maxDeepTickers),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
			defer stop()

			out := cmd.OutOrStdout()
			interactive := isatty.IsTerminal(os.Stdout.Fd())
			line := progressLine(out, interactive, "detalhe:")

			tickers := make([]string, len(args))
			for i, arg := range args {
				tickers[i] = strings.ToUpper(arg)
			}

			report, err := collect.Deep(ctx, collect.DeepConfig{
				DB:         deps.DB,
				Detail:     deps.Detail,
				Tickers:    tickers,
				Force:      force,
				Now:        time.Now,
				OnProgress: func(p collect.Progress) { line(p.Done, p.Total) },
			})
			endProgress(out, interactive)
			if err != nil {
				return err
			}

			for _, outcome := range report.Outcomes {
				fmt.Fprintln(out, outcomeLine(outcome))
			}
			if report.Cancelled {
				fmt.Fprintf(out, "⚠ coleta interrompida · %d de %d · rode de novo para continuar\n",
					len(report.Outcomes), len(tickers))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "ignora o cache e rebate na fonte")
	return cmd
}

func outcomeLine(o collect.TickerOutcome) string {
	if o.Status == collect.StatusOK {
		return fmt.Sprintf("✓ %s · detalhe e proventos coletados", o.Ticker)
	}
	return fmt.Sprintf("✗ %s · %s", o.Ticker, o.Reason)
}
