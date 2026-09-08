package derive

import (
	"github.com/marlliton/goinvest/internal/domain"
	"github.com/marlliton/goinvest/internal/norm"
)

func isInputSuspect(metric domain.MetricID, v *float64) bool {
	if v == nil {
		return true
	}
	return norm.IsAbsenceSentinel(metric, *v)
}
