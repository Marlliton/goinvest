package architecture_test

import (
	"os/exec"
	"strings"
	"testing"

	// Sem este import o cache de teste do Go serve um resultado obsoleto: o
	// grafo de dependências é lido por um subprocesso `go list`, invisível para
	// o cache. Todo pacote vigiado aqui precisa do seu import em branco.
	_ "github.com/marlliton/goinvest/internal/app"
	_ "github.com/marlliton/goinvest/internal/catalog"
	_ "github.com/marlliton/goinvest/internal/derive"
	_ "github.com/marlliton/goinvest/internal/domain"
	_ "github.com/marlliton/goinvest/internal/fetch"
)

const modulePath = "github.com/marlliton/goinvest"

// Alguns destes pacotes internos ainda não existem; estão na lista para o dia
// em que existirem.
var forbiddenForCore = []string{
	"database/sql",
	"net/http",
	"modernc.org/sqlite",
	modulePath + "/internal/store",
	modulePath + "/internal/fetch",
	modulePath + "/internal/provider",
}

func TestDomainHasNoInfraImports(t *testing.T) {
	requireNoImports(t, modulePath+"/internal/domain", forbiddenForCore)
}

func TestCatalogHasNoInfraImports(t *testing.T) {
	requireNoImports(t, modulePath+"/internal/catalog", forbiddenForCore)
}

// derive é análise pura: recebe MetricSet e devolve MetricSet. Também não pode
// alcançar o catálogo, senão a unidade do derivado passaria a ter duas fontes.
func TestDeriveHasNoInfraImports(t *testing.T) {
	requireNoImports(t, modulePath+"/internal/derive",
		append(forbiddenForCore, modulePath+"/internal/catalog"))
}

// A interface Cache é declarada pelo consumidor justamente para que fetch e
// store possam evoluir sem se conhecer. Sem este teste a dependência entra por
// conveniência no primeiro plano que precisar de cache concreto.
func TestFetchDoesNotImportStore(t *testing.T) {
	requireNoImports(t, modulePath+"/internal/fetch", []string{modulePath + "/internal/store"})
}

// O grep na assinatura de Show prova só o arquivo. A promessa de leitura
// offline é sobre o grafo inteiro: é aqui que ela vira erro de teste.
func TestAppCannotReachTheNetwork(t *testing.T) {
	requireNoImports(t, modulePath+"/internal/app", []string{
		"net/http",
		modulePath + "/internal/fetch",
		modulePath + "/internal/provider",
	})
}

func requireNoImports(t *testing.T, pkg string, forbidden []string) {
	t.Helper()
	deps := goListDeps(t, pkg+"/...")
	requirePackagePresent(t, deps, pkg)

	for _, f := range forbidden {
		for _, dep := range deps {
			// Casar por prefixo: internal/store/gen é infraestrutura tanto
			// quanto internal/store, e a lista não pode depender de alguém
			// lembrar de estendê-la a cada subpacote novo.
			if dep == f || strings.HasPrefix(dep, f+"/") {
				t.Errorf("%s imports %s (transitively)", pkg, dep)
			}
		}
	}
}

// requirePackagePresent evita o falso verde: `go list` sobre um padrão que não
// casa com nenhum pacote sai com status 0 e saída vazia, e o laço de
// verificação passa sem verificar nada.
func requirePackagePresent(t *testing.T, deps []string, pkg string) {
	t.Helper()
	for _, dep := range deps {
		if dep == pkg {
			return
		}
	}
	t.Fatalf("go list -deps did not return %s: the pattern matched no package", pkg)
}

// O padrão precisa vir qualificado pelo module path: o cwd do teste é o
// diretório do próprio pacote, onde um "./..." resolveria para a subárvore errada.
func goListDeps(t *testing.T, pattern string) []string {
	t.Helper()
	out, err := exec.Command("go", "list", "-deps", pattern).Output()
	if err != nil {
		t.Fatalf("go list -deps %s failed: %v", pattern, err)
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}
