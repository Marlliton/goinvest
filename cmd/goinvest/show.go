package main

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/marlliton/goinvest/internal/app"
	"github.com/spf13/cobra"
)

var errNoLocalData = errors.New("Nenhum dado local. Rode 'goinvest sync' primeiro.")

func newShowCmd(deps rootDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "show TICKER",
		Short: "Mostra os indicadores já coletados de um ativo",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ticker := strings.ToUpper(args[0])

			report, err := app.Show(cmd.Context(), deps.DB, deps.Catalog, ticker, time.Now)
			if err != nil {
				if errors.Is(err, app.ErrNoData) {
					return errNoLocalData
				}
				return err
			}

			fmt.Fprint(cmd.OutOrStdout(), app.RenderText(report))
			return nil
		},
	}
}
