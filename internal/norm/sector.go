package norm

import "strings"

// O espaço antes do ponto é sujeira da B3 ("Motores . Compressores"), não
// parte do nome do segmento.
var sectorCleaner = strings.NewReplacer(" . ", ". ")

func CleanSector(s string) string {
	return strings.Join(strings.Fields(sectorCleaner.Replace(s)), " ")
}
