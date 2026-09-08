package norm_test

import (
	"testing"

	"github.com/marlliton/goinvest/internal/norm"
	"github.com/stretchr/testify/require"
)

func TestFoldUpper(t *testing.T) {
	cases := map[string]string{
		"JRS CAP PRÓPRIO": "JRS CAP PROPRIO",
		"JRS CAP PROPRIO": "JRS CAP PROPRIO",
		"Dividendo":       "DIVIDENDO",
		"  Juros \n":      "JUROS",
		"Rendimento":      "RENDIMENTO",
		"":                "",
	}

	for in, want := range cases {
		require.Equal(t, want, norm.FoldUpper(in), "entrada %q", in)
	}
}
