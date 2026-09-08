package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/marlliton/goinvest/internal/app"
	"github.com/spf13/cobra"
)

// De 3 a 8: abaixo de três não é comparação, e acima de oito a tabela deixa de
// caber em tela nenhuma.
const (
	minCompareTickers = 3
	maxCompareTickers = 8
)

func newCompareCmd(deps rootDeps) *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "compare TICKER TICKER TICKER...",
		Short: "Compara de 3 a 8 ativos lado a lado",
		Args:  cobra.RangeArgs(minCompareTickers, maxCompareTickers),
		RunE: func(cmd *cobra.Command, args []string) error {
			tickers := make([]string, 0, len(args))
			for _, arg := range args {
				tickers = append(tickers, strings.ToUpper(arg))
			}

			// Ticker que não entrou sai no rodapé do relatório com o motivo:
			// derrubar o comando por causa de um dos oito é o caro.
			report, err := app.Compare(cmd.Context(), deps.DB, deps.Catalog, tickers, time.Now)
			if err != nil {
				return err
			}

			if !asJSON {
				fmt.Fprint(cmd.OutOrStdout(), app.RenderCompareText(report))
				return nil
			}

			raw, err := app.RenderCompareJSON(report)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(raw))
			return nil
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "imprime o resultado como JSON (schema_version 1)")
	return cmd
}
