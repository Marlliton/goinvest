package norm

import "github.com/marlliton/goinvest/internal/domain"

// Valor numericamente válido que é, na verdade, o código de "não sei" daquela
// métrica: o Fundamentus publica EV/EBITDA como 0,00 para bancos, onde o
// indicador não se aplica.
func IsAbsenceSentinel(metric domain.MetricID, v float64) bool {
	switch metric {
	case "ev_ebitda":
		return v == 0
	default:
		return false
	}
}
