// Package architecture_test guarda as invariantes estruturais da fase — asserções
// sobre o grafo de dependências entre pacotes, não sobre comportamento de código.
//
// O Critério de Sucesso 5 do ROADMAP exige que "nova fonte sem alterar núcleo"
// (DATA-07) seja verificado por teste de dependência entre pacotes, não por
// revisão manual de PR. É o que este arquivo faz, em milissegundos, a cada
// `go test ./...`.
package architecture_test

import (
	"os/exec"
	"strings"
	"testing"

	// Import em branco deliberado, não acidental: sem ele o cache de teste do Go
	// serve um "ok (cached)" para este pacote mesmo depois de internal/domain
	// passar a importar net/http. O cache indexa os arquivos que o binário de
	// teste compila, e este teste lê o grafo de dependências por um subprocesso
	// `go list` — invisível para o cache. Importar os pacotes vigiados torna
	// qualquer mudança neles uma invalidação de cache, que é o que mantém o
	// guarda vivo num `go test ./...` sem -count=1.
	//
	// Todo pacote novo adicionado a um teste de fronteira aqui precisa ganhar o
	// seu import em branco junto.
	_ "github.com/marlliton/goinvest/internal/domain"
)

const modulo = "github.com/marlliton/goinvest"

// proibidosParaNucleo lista os pacotes de infraestrutura que o núcleo
// (domain, e mais adiante catalog) nunca pode importar, direta ou
// transitivamente. Os três pacotes internos ainda não existem nesta altura da
// fase — estão aqui preventivamente, e `go list -deps` simplesmente não os
// encontrará até que existam.
var proibidosParaNucleo = []string{
	"database/sql",
	"net/http",
	"modernc.org/sqlite",
	modulo + "/internal/store",
	modulo + "/internal/fetch",
	modulo + "/internal/provider",
}

func TestFronteiraDomainSemInfra(t *testing.T) {
	deps := goListDeps(t, modulo+"/internal/domain/...")
	exigirPacotePresente(t, deps, modulo+"/internal/domain")
	for _, proibido := range proibidosParaNucleo {
		for _, d := range deps {
			if d == proibido {
				t.Errorf("internal/domain importa %s (transitivamente) — viola domain sem dependências de infraestrutura", proibido)
			}
		}
	}
}

// exigirPacotePresente protege contra o modo de falha mais perigoso deste
// arquivo: um `go list -deps` sobre um padrão que não casa com nenhum pacote
// sai com status 0 e saída vazia, e o laço de verificação sobre uma lista vazia
// passa sem verificar nada. Um teste de fronteira vacuamente verde é pior que
// nenhum teste — anuncia uma garantia que não está checando.
func exigirPacotePresente(t *testing.T, deps []string, pacote string) {
	t.Helper()
	for _, d := range deps {
		if d == pacote {
			return
		}
	}
	t.Fatalf("go list -deps não devolveu %s — o padrão não casou com nenhum pacote e o teste passaria sem verificar nada", pacote)
}

// goListDeps devolve o fecho transitivo de dependências do padrão de pacote
// informado. O padrão é qualificado pelo module path (não relativo): os testes
// rodam com o cwd no diretório do próprio pacote, onde um padrão `./...`
// resolveria para a subárvore errada.
func goListDeps(t *testing.T, pattern string) []string {
	t.Helper()
	out, err := exec.Command("go", "list", "-deps", pattern).Output()
	if err != nil {
		t.Fatalf("go list -deps %s falhou: %v", pattern, err)
	}
	limpo := strings.TrimSpace(string(out))
	if limpo == "" {
		return nil
	}
	return strings.Split(limpo, "\n")
}
