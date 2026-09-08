package norm

import (
	"strings"
	"time"
)

const brDateLayout = "02/01/2006"

// O "-" que a fonte escreve onde nunca publicou data volta como ausência, não
// como erro.
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
