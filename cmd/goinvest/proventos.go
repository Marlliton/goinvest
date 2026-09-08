package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/marlliton/goinvest/internal/app"
	"github.com/spf13/cobra"
)

func newProventosCmd(deps rootDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "proventos TICKER",
		Short: "Lista os proventos já coletados de um ativo, por ação",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ticker := strings.ToUpper(args[0])

			view, err := app.Dividends(cmd.Context(), deps.DB, ticker)
			if err != nil {
				switch {
				case errors.Is(err, app.ErrNoData):
					return errNoLocalData
				case errors.Is(err, app.ErrNoDividends):
					return fmt.Errorf("nenhum provento de %s. Rode 'goinvest detalhar %s' primeiro", ticker, ticker)
				}
				return err
			}

			fmt.Fprint(cmd.OutOrStdout(), app.RenderDividends(view))
			return nil
		},
	}
}
