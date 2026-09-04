package identity

import "strings"

const (
	brazilPrefix  = "BR"
	issuerCodeEnd = 6
	fiiSuffix     = "11"
)

// Heurística, não mapeamento oficial: acerta cerca de três em cada quatro
// fundos, e o não-casamento é resultado que o chamador precisa tratar.
func TickerFromISIN(isin string) (ticker string, ok bool) {
	if len(isin) < issuerCodeEnd || !strings.HasPrefix(isin, brazilPrefix) {
		return "", false
	}
	return isin[len(brazilPrefix):issuerCodeEnd] + fiiSuffix, true
}
