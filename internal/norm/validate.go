package norm

import "github.com/marlliton/goinvest/internal/domain"

// IsSentinelaAusencia diz se um valor numericamente válido é, na verdade, o
// código de "não sei" daquela métrica.
//
// O parser genérico está certo em devolver (0, true) para "0,00" — leu um
// número. Mas o Fundamentus publica EV/EBITDA como 0,00 para bancos, onde o
// indicador não se aplica: aceitar esse zero como valor real contamina médias
// setoriais e acende o semáforo verde no lugar errado.
//
// O switch é a forma extensível de propósito: sentinelas são descobertos um a
// um, por métrica, conforme novas fontes entram. Um if isolado convidaria o
// próximo caso a virar outro if isolado em outro lugar.
func IsSentinelaAusencia(metric domain.MetricID, v float64) bool {
	switch metric {
	case "ev_ebitda":
		return v == 0
	default:
		return false
	}
}
