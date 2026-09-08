package main

import (
	"fmt"
	"io"

	"github.com/marlliton/goinvest/internal/registry"
)

// Fora de TTY cada atualização vira uma linha: o \r que reescreve a linha no
// terminal vira lixo num arquivo de log.
func progressLine(out io.Writer, interactive bool, prefix string) func(done, total int) {
	format := prefix + " %d/%d\n"
	if interactive {
		format = "\r" + prefix + " %d/%d"
	}
	return func(done, total int) {
		fmt.Fprintf(out, format, done, total)
	}
}

func progressWriter(out io.Writer, interactive bool, label string) func(registry.Progress) {
	line := progressLine(out, interactive, "cadastro: "+label)
	return func(p registry.Progress) { line(p.Done, p.Total) }
}

func endProgress(out io.Writer, interactive bool) {
	if interactive {
		fmt.Fprintln(out)
	}
}
