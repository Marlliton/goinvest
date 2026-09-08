package norm

import (
	"strings"
	"time"
)

const brDateLayout = "02/01/2006"

// ParseBRDate lê uma data no formato dd/mm/aaaa. O bool separa ausência de
// data zero, pelo mesmo motivo que ParseBRNumber separa ausência de zero: a
// fonte escreve "-" nos eventos antigos que nunca tiveram data publicada.
func ParseBRDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	switch s {
	case "", "-", "N/A", "n/a":
		return time.Time{}, false
	}

	d, err := time.Parse(brDateLayout, s)
	if err != nil {
		return time.Time{}, false
	}
	return d, true
}
