package bazin

import (
	"math"
	"time"

	"github.com/marlliton/goinvest/internal/domain"
)

// CurrentYearConcentration reaplica o critério de atypicalEventShare do
// Compute sobre o ano corrente, que Compute descarta de propósito por ainda
// estar aberto.
func CurrentYearConcentration(events []domain.DividendEvent, now time.Time) (share float64, ok bool) {
	var total, largest float64
	for _, e := range events {
		if e.ExDate.Year() != now.Year() {
			continue
		}
		v, valid := netPerShare(e)
		if !valid {
			continue
		}
		total += v
		largest = math.Max(largest, v)
	}
	if total <= 0 {
		return 0, false
	}
	return largest / total, true
}
