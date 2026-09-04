package main

import (
	"errors"
	"fmt"

	"github.com/marlliton/goinvest/internal/app"
	"github.com/spf13/cobra"
)

func newSectorsCmd(deps rootDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "sectors [setor]",
		Short: "Lista a taxonomia setorial e a quantidade de ativos líquidos",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()

			if len(args) == 0 {
				groups, err := app.Sectors(cmd.Context(), deps.DB)
				if err != nil {
					return err
				}
				fmt.Fprint(out, app.RenderSectors(groups))
				return nil
			}

			sector := args[0]
			groups, err := app.SectorsDescend(cmd.Context(), deps.DB, sector)
			if err != nil {
				if errors.Is(err, app.ErrSectorNotFound) {
					return fmt.Errorf("setor %q não encontrado. Rode 'goinvest sectors' para ver os setores disponíveis", sector)
				}
				return err
			}
			fmt.Fprint(out, app.RenderSectorsDescend(sector, groups))
			return nil
		},
	}
}
