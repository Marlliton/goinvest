package derive

import (
	"github.com/marlliton/goinvest/internal/domain"
	"github.com/marlliton/goinvest/internal/norm"
)

// A regra de sentinela é de norm: duplicá-la aqui faria um sentinela novo
// passar despercebido justamente nos derivados, que são os mais frágeis a ele.
func isInputSuspect(metric domain.MetricID, v *float64) bool {
	if v == nil {
		return true
	}
	return norm.IsAbsenceSentinel(metric, *v)
}
