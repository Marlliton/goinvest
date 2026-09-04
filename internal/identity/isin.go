package identity

import "strings"

const (
	brazilPrefix  = "BR"
	issuerCodeEnd = 6
	fiiSuffix     = "11"
)

// TickerFromISIN é heurística, não mapeamento oficial: acerta cerca de três em
// cada quatro fundos. Quem chama precisa tratar o não-casamento como resultado.
func TickerFromISIN(isin string) (ticker string, ok bool) {
	if len(isin) < issuerCodeEnd || !strings.HasPrefix(isin, brazilPrefix) {
		return "", false
	}
	return isin[len(brazilPrefix):issuerCodeEnd] + fiiSuffix, true
}
