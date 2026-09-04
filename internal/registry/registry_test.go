package registry_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/marlliton/goinvest/internal/domain"
	"github.com/marlliton/goinvest/internal/identity"
	"github.com/marlliton/goinvest/internal/registry"
	"github.com/marlliton/goinvest/internal/store"
	"github.com/stretchr/testify/require"
)

var seededAt = time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)

type stubIdentity struct {
	companies   []identity.CompanyRef
	details     map[string]identity.CompanyDetail
	detailErr   map[string]error
	detailCalls int
	onDetail    func(codeCVM string)
}

func (s *stubIdentity) Name() string { return "stub" }

func (s *stubIdentity) Companies(context.Context, bool) ([]identity.CompanyRef, error) {
	return s.companies, nil
}

func (s *stubIdentity) Detail(_ context.Context, codeCVM string, _ bool) (identity.CompanyDetail, error) {
	s.detailCalls++
	if s.onDetail != nil {
		s.onDetail(codeCVM)
	}
	if err, ok := s.detailErr[codeCVM]; ok {
		return identity.CompanyDetail{}, err
	}
	d, ok := s.details[codeCVM]
	if !ok {
		return identity.CompanyDetail{}, errors.New("codeCVM desconhecido")
	}
	return d, nil
}

func openDB(t *testing.T) *store.DB {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "goinvest.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	return db
}

func seedActive(t *testing.T, db *store.DB, tickers ...string) {
	t.Helper()
	ctx := t.Context()
	for _, ticker := range tickers {
		require.NoError(t, db.UpsertAsset(ctx, ticker, domain.ClassStock, "", seededAt))
		a, found, err := db.GetAsset(ctx, ticker)
		require.NoError(t, err)
		require.True(t, found)
		require.NoError(t, db.UpdateAssetLiquidity(ctx, a.AssetID, true, seededAt))
	}
}

func detail(code, codeCVM, cnpj, classification string, others ...string) identity.CompanyDetail {
	d := identity.CompanyDetail{
		Code:                   code,
		CodeCVM:                codeCVM,
		CNPJ:                   cnpj,
		IndustryClassification: classification,
	}
	for _, o := range others {
		d.OtherCodes = append(d.OtherCodes, identity.CompanyCode{Code: o, ISIN: "BR" + o + "ACNOR0"})
	}
	return d
}

func newStub() *stubIdentity {
	return &stubIdentity{
		companies: []identity.CompanyRef{
			{IssuingCompany: "WEGE", CodeCVM: "5410"},
			{IssuingCompany: "ITUB", CodeCVM: "19348"},
			{IssuingCompany: "TAEE", CodeCVM: "20257"},
		},
		details: map[string]identity.CompanyDetail{
			"5410": detail("WEGE3", "5410", "84429695000111",
				"Bens Industriais / Máquinas e Equipamentos / Motores . Compressores e Outros", "WEGE3"),
			"19348": detail("ITUB4", "19348", "60872504000123",
				"Financeiro / Intermediários Financeiros / Bancos", "ITUB3", "ITUB4"),
			"20257": detail("TAEE11", "20257", "07859971000130",
				"Utilidade Pública / Energia Elétrica / Energia Elétrica", "TAEE11"),
		},
		detailErr: map[string]error{},
	}
}

func TestRunAppliesBatchesTransactionally(t *testing.T) {
	db := openDB(t)
	seedActive(t, db, "WEGE3", "ITUB4", "TAEE11")

	report, err := registry.Run(t.Context(), registry.Config{
		DB: db, Identity: newStub(), BatchSize: 2, Now: func() time.Time { return seededAt },
	})
	require.NoError(t, err)
	require.Equal(t, 3, report.Total)
	require.Equal(t, 3, report.Matched)
	require.Zero(t, report.Unmatched)
	require.Equal(t, registry.StatusOK, report.Status)

	a, _, err := db.GetAsset(t.Context(), "WEGE3")
	require.NoError(t, err)
	require.Equal(t, "Bens Industriais", a.Sector)
	require.Equal(t, "Máquinas e Equipamentos", a.Subsector)
	require.Equal(t, "Motores. Compressores e Outros", a.Segment, "o rótulo chega limpo")
	require.Equal(t, "84429695000111", a.CNPJ)
	require.Equal(t, "5410", a.CDCVM)
	require.Equal(t, "b3", a.SectorSrc)

	taee, _, err := db.GetAsset(t.Context(), "TAEE11")
	require.NoError(t, err)
	require.Equal(t, "Utilidade Pública", taee.Sector)
}

func TestRunCancellationPreservesCommittedBatches(t *testing.T) {
	db := openDB(t)
	seedActive(t, db, "WEGE3", "ITUB4", "TAEE11")

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// Os tickers são processados em ordem alfabética (ITUB4, TAEE11, WEGE3):
	// cancelar no terceiro deixa o primeiro lote inteiro já commitado.
	stub := newStub()
	seen := 0
	stub.onDetail = func(string) {
		seen++
		if seen == 3 {
			cancel()
		}
	}

	report, err := registry.Run(ctx, registry.Config{
		DB: db, Identity: stub, BatchSize: 2, Now: func() time.Time { return seededAt },
	})
	require.NoError(t, err)
	require.True(t, report.Cancelled)
	require.Equal(t, registry.StatusCancelled, report.Status)
	require.Equal(t, 2, report.Matched, "o lote fechado antes do cancelamento foi aplicado")
	require.Zero(t, report.Unmatched, "interrompido não é o mesmo que sem correspondência")

	a, _, err := db.GetAsset(t.Context(), "ITUB4")
	require.NoError(t, err)
	require.Equal(t, "Financeiro", a.Sector)

	last, _, err := db.GetAsset(t.Context(), "WEGE3")
	require.NoError(t, err)
	require.Empty(t, last.Sector, "o ticker interrompido não foi gravado")

	require.Equal(t, registry.StatusCancelled, latestRunStatus(t, db, "b3:listed_companies"),
		"o run fecha mesmo com o contexto cancelado")
}

func TestRunUnmatchedTickerDoesNotAbortRun(t *testing.T) {
	db := openDB(t)
	seedActive(t, db, "WEGE3", "NADA4", "ITUB4")

	stub := newStub()
	stub.detailErr["19348"] = errors.New("b3: detail 19348: status 500")

	report, err := registry.Run(t.Context(), registry.Config{
		DB: db, Identity: stub, Now: func() time.Time { return seededAt },
	})
	require.NoError(t, err)
	require.Equal(t, 3, report.Total)
	require.Equal(t, 1, report.Matched)
	require.Equal(t, 2, report.Unmatched, "raiz sem correspondência e falha de detalhe")
	require.Equal(t, registry.StatusPartial, report.Status)

	a, _, err := db.GetAsset(t.Context(), "WEGE3")
	require.NoError(t, err)
	require.Equal(t, "Bens Industriais", a.Sector, "a falha de um ticker não impede os outros")
}

func TestRunRejectsTickerNotConfirmedByOtherCodes(t *testing.T) {
	db := openDB(t)
	seedActive(t, db, "WEGE4")

	report, err := registry.Run(t.Context(), registry.Config{
		DB: db, Identity: newStub(), Now: func() time.Time { return seededAt },
	})
	require.NoError(t, err)
	require.Equal(t, 1, report.Unmatched)
	require.Zero(t, report.Matched)

	a, _, err := db.GetAsset(t.Context(), "WEGE4")
	require.NoError(t, err)
	require.Empty(t, a.Sector)
}

func TestRunSkipsInactiveAssets(t *testing.T) {
	db := openDB(t)
	seedActive(t, db, "WEGE3")

	ctx := t.Context()
	require.NoError(t, db.UpsertAsset(ctx, "ITUB4", domain.ClassStock, "", seededAt))
	a, _, err := db.GetAsset(ctx, "ITUB4")
	require.NoError(t, err)
	require.NoError(t, db.UpdateAssetLiquidity(ctx, a.AssetID, false, seededAt))

	stub := newStub()
	report, err := registry.Run(ctx, registry.Config{
		DB: db, Identity: stub, Now: func() time.Time { return seededAt },
	})
	require.NoError(t, err)
	require.Equal(t, 1, report.Total)
	require.Equal(t, 1, stub.detailCalls)
}

func TestRunReportsProgress(t *testing.T) {
	db := openDB(t)
	seedActive(t, db, "WEGE3", "ITUB4", "TAEE11")

	var last registry.Progress
	_, err := registry.Run(t.Context(), registry.Config{
		DB: db, Identity: newStub(), BatchSize: 2,
		Now:        func() time.Time { return seededAt },
		OnProgress: func(p registry.Progress) { last = p },
	})
	require.NoError(t, err)
	require.Equal(t, 3, last.Total)
	require.Equal(t, 3, last.Done)
}

func TestRunRequiresDBAndIdentity(t *testing.T) {
	_, err := registry.Run(t.Context(), registry.Config{Identity: newStub()})
	require.Error(t, err)

	_, err = registry.Run(t.Context(), registry.Config{DB: openDB(t)})
	require.Error(t, err)
}

type stubFIISource struct {
	byCNPJ   map[string]string
	segments map[string]string
}

func (stubFIISource) Name() string { return "stub" }

func (s stubFIISource) ISINByCNPJ(context.Context, bool) (map[string]string, error) {
	return s.byCNPJ, nil
}

func (s stubFIISource) Segments(context.Context, bool) (map[string]string, error) {
	return s.segments, nil
}

func seedFII(t *testing.T, db *store.DB, tickers ...string) {
	t.Helper()
	for _, ticker := range tickers {
		require.NoError(t, db.UpsertAsset(t.Context(), ticker, domain.ClassFII, "", seededAt))
	}
}

func TestRunFIIMatchesByHeuristicAndReportsUnmatched(t *testing.T) {
	db := openDB(t)
	seedFII(t, db, "FVPQ11")

	src := stubFIISource{
		byCNPJ: map[string]string{
			"00332266000131": "BRFVPQCTF015",
			"11111111000111": "BRNADACTF015",
			"22222222000122": "US0378331005",
		},
		segments: map[string]string{"FVPQ11": "Shoppings"},
	}

	report, err := registry.RunFII(t.Context(), registry.FIIConfig{
		DB: db, CVM: src, Fundamentus: src, Now: func() time.Time { return seededAt },
	})
	require.NoError(t, err)
	require.Equal(t, 1, report.Matched)
	require.Equal(t, 2, report.Unmatched, "ticker candidato inexistente e ISIN fora da convenção")
	require.Equal(t, registry.StatusPartial, report.Status)

	a, _, err := db.GetAsset(t.Context(), "FVPQ11")
	require.NoError(t, err)
	require.Equal(t, "BRFVPQCTF015", a.ISIN)
	require.Equal(t, "00332266000131", a.CNPJ)
	require.Equal(t, "Shoppings", a.Sector)
	require.Equal(t, "fundamentus:segmento", a.SectorSrc,
		"a marca de origem é o que deixa a Fase 5 substituir sem ambiguidade")
	require.Empty(t, a.Subsector, "a taxonomia de FII do Fundamentus tem um nível só")
}

func TestRunFIIIncludesInactiveAssets(t *testing.T) {
	db := openDB(t)
	seedFII(t, db, "FVPQ11")

	a, _, err := db.GetAsset(t.Context(), "FVPQ11")
	require.NoError(t, err)
	require.NoError(t, db.UpdateAssetLiquidity(t.Context(), a.AssetID, false, seededAt))

	report, err := registry.RunFII(t.Context(), registry.FIIConfig{
		DB: db,
		CVM: stubFIISource{
			byCNPJ:   map[string]string{"00332266000131": "BRFVPQCTF015"},
			segments: map[string]string{"FVPQ11": "Shoppings"},
		},
		Fundamentus: stubFIISource{segments: map[string]string{"FVPQ11": "Shoppings"}},
		Now:         func() time.Time { return seededAt },
	})
	require.NoError(t, err)
	require.Equal(t, 1, report.Matched)
}

func latestRunStatus(t *testing.T, db *store.DB, source string) string {
	t.Helper()
	var status string
	require.NoError(t, db.QueryRow(
		`SELECT status FROM collection_run WHERE source = ? ORDER BY id DESC LIMIT 1`,
		source).Scan(&status))
	return status
}
