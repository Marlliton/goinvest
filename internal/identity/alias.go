// Package identity resolve qual ativo do mundo real está por trás de um
// código, sem tocar em rede nem em banco.
package identity

func FractionalAlias(ticker string) string {
	return ticker + "F"
}
