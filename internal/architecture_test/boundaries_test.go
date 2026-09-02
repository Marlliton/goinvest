package architecture_test

import (
	"os/exec"
	"strings"
	"testing"

	// Sem este import o cache de teste do Go serve um resultado obsoleto: o
	// grafo de dependências é lido por um subprocesso `go list`, invisível para
	// o cache. Todo pacote vigiado aqui precisa do seu import em branco.
	_ "github.com/marlliton/goinvest/internal/catalog"
	_ "github.com/marlliton/goinvest/internal/domain"
)

const modulePath = "github.com/marlliton/goinvest"

// Os três pacotes internos ainda não existem; estão na lista para o dia em que
// existirem.
var forbiddenForCore = []string{
	"database/sql",
	"net/http",
	"modernc.org/sqlite",
	modulePath + "/internal/store",
	modulePath + "/internal/fetch",
	modulePath + "/internal/provider",
}

func TestDomainHasNoInfraImports(t *testing.T) {
	deps := goListDeps(t, modulePath+"/internal/domain/...")
	requirePackagePresent(t, deps, modulePath+"/internal/domain")

	for _, forbidden := range forbiddenForCore {
		for _, dep := range deps {
			if dep == forbidden {
				t.Errorf("internal/domain imports %s (transitively)", forbidden)
			}
		}
	}
}

func TestCatalogHasNoInfraImports(t *testing.T) {
	deps := goListDeps(t, modulePath+"/internal/catalog/...")
	requirePackagePresent(t, deps, modulePath+"/internal/catalog")

	for _, forbidden := range forbiddenForCore {
		for _, dep := range deps {
			if dep == forbidden {
				t.Errorf("internal/catalog imports %s (transitively)", forbidden)
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
