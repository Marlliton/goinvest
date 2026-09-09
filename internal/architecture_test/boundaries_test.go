package architecture_test

import (
	"os/exec"
	"strings"
	"testing"

	// O grafo de dependências é lido por um subprocesso `go list`, invisível
	// para o cache de teste. Sem o import em branco o pacote vigiado sai com
	// resultado obsoleto.
	_ "github.com/marlliton/goinvest/internal/app"
	_ "github.com/marlliton/goinvest/internal/bazin"
	_ "github.com/marlliton/goinvest/internal/catalog"
	_ "github.com/marlliton/goinvest/internal/collect"
	_ "github.com/marlliton/goinvest/internal/config"
	_ "github.com/marlliton/goinvest/internal/derive"
	_ "github.com/marlliton/goinvest/internal/domain"
	_ "github.com/marlliton/goinvest/internal/evaluate"
	_ "github.com/marlliton/goinvest/internal/fetch"
	_ "github.com/marlliton/goinvest/internal/identity"
	_ "github.com/marlliton/goinvest/internal/norm"
	_ "github.com/marlliton/goinvest/internal/provider/b3"
	_ "github.com/marlliton/goinvest/internal/provider/bcb"
	_ "github.com/marlliton/goinvest/internal/provider/cvm"
	_ "github.com/marlliton/goinvest/internal/provider/tesouro"
	_ "github.com/marlliton/goinvest/internal/registry"
)

const modulePath = "github.com/marlliton/goinvest"

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

// O catálogo entra na lista porque a unidade do derivado passaria a ter duas
// fontes.
func TestDeriveHasNoInfraImports(t *testing.T) {
	requireNoImports(t, modulePath+"/internal/derive",
		append(forbiddenForCore, modulePath+"/internal/catalog", modulePath+"/internal/config"))
}

func TestBazinHasNoInfraImports(t *testing.T) {
	requireNoImports(t, modulePath+"/internal/bazin",
		append(forbiddenForCore, modulePath+"/internal/catalog", modulePath+"/internal/config"))
}

// O texto do catálogo chega pronto no Input: a alternativa seria a regra e a
// redação da regra evoluírem em dois lugares.
func TestEvaluateHasNoInfraImports(t *testing.T) {
	requireNoImports(t, modulePath+"/internal/evaluate",
		append(forbiddenForCore, modulePath+"/internal/catalog", modulePath+"/internal/config"))
}

func TestIdentityHasNoInfraImports(t *testing.T) {
	requireNoImports(t, modulePath+"/internal/identity", forbiddenForCore)
}

// A interface Cache é declarada pelo consumidor para que fetch e store possam
// evoluir sem se conhecer.
func TestFetchDoesNotImportStore(t *testing.T) {
	requireNoImports(t, modulePath+"/internal/fetch", []string{modulePath + "/internal/store"})
}

// A interface provider fica de fora da lista de propósito: ela não disca nada,
// e app precisa dela desde Sync.
func TestAppCannotReachTheNetwork(t *testing.T) {
	requireNoImports(t, modulePath+"/internal/app", []string{
		"net/http",
		modulePath + "/internal/fetch",
		modulePath + "/internal/provider/fundamentus",
		modulePath + "/internal/provider/bcb",
	})
}

func TestCollectDependsOnlyOnProviderInterface(t *testing.T) {
	requireNoImports(t, modulePath+"/internal/collect",
		[]string{modulePath + "/internal/provider/fundamentus"})
}

func TestRegistryDependsOnlyOnProviderInterface(t *testing.T) {
	requireNoImports(t, modulePath+"/internal/registry",
		[]string{modulePath + "/internal/provider/b3"})
}

func requireNoImports(t *testing.T, pkg string, forbidden []string) {
	t.Helper()
	deps := goListDeps(t, pkg+"/...")
	requirePackagePresent(t, deps, pkg)

	for _, f := range forbidden {
		for _, dep := range deps {
			if dep == f || strings.HasPrefix(dep, f+"/") {
				t.Errorf("%s imports %s (transitively)", pkg, dep)
			}
		}
	}
}

// `go list` sobre um padrão que não casa com nenhum pacote sai com status 0 e
// saída vazia, e o laço de verificação passaria sem verificar nada.
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
// diretório do próprio pacote, e "./..." resolveria para a subárvore errada.
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
