package store

import (
	"database/sql"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/marlliton/goinvest/internal/domain"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
)

func openTemp(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "goinvest.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	return db
}

func seedAsset(t *testing.T, db *DB, ticker string, class domain.AssetClass) {
	t.Helper()
	require.NoError(t, db.UpsertAsset(t.Context(), ticker, class, ticker+" S.A.", time.Now().UTC()))
}

func obs(ticker string, metric domain.MetricID, value *float64, fetchedAt time.Time) domain.Observation {
	return domain.Observation{
		Ticker:     ticker,
		Metric:     metric,
		PeriodKind: "spot",
		PeriodEnd:  fetchedAt.Truncate(24 * time.Hour),
		Value:      value,
		Unit:       domain.UnitRatio,
		Source:     "fundamentus:resultado",
		FetchedAt:  fetchedAt,
	}
}

func ptr(v float64) *float64 { return &v }

func TestMigrationsUpDownUp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "goinvest.db")
	db, err := Open(path)
	require.NoError(t, err)
	defer db.Close()

	goose.SetBaseFS(migrationsFS)
	require.NoError(t, goose.SetDialect("sqlite3"))
	require.NoError(t, goose.Down(db.DB, "migrations"))
	require.NoError(t, goose.Up(db.DB, "migrations"))

	var n int
	require.NoError(t, db.QueryRow(
		`SELECT count(*) FROM sqlite_master WHERE type='table'
		 AND name IN ('asset','asset_alias','collection_run','observation','raw_doc')`).Scan(&n))
	require.Equal(t, 5, n)
}

// Ativo nasce ativo: só o sync tem dado para dizer o contrário, e ele roda
// depois do cadastro.
func TestUpdateAssetLiquidity(t *testing.T) {
	db := openTemp(t)
	ctx := t.Context()
	seedAsset(t, db, "WEGE3", domain.ClassStock)

	a, _, err := db.GetAsset(ctx, "WEGE3")
	require.NoError(t, err)
	require.True(t, a.IsActive)
	require.Nil(t, a.LastLiquidAt)

	liquidAt := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	require.NoError(t, db.UpdateAssetLiquidity(ctx, a.AssetID, true, liquidAt))

	a, _, err = db.GetAsset(ctx, "WEGE3")
	require.NoError(t, err)
	require.True(t, a.IsActive)
	require.NotNil(t, a.LastLiquidAt)
	require.Equal(t, liquidAt, a.LastLiquidAt.UTC())

	require.NoError(t, db.UpdateAssetLiquidity(ctx, a.AssetID, false, liquidAt.AddDate(0, 1, 0)))

	a, _, err = db.GetAsset(ctx, "WEGE3")
	require.NoError(t, err)
	require.False(t, a.IsActive)
	require.Equal(t, liquidAt, a.LastLiquidAt.UTC(), "ficar inativo não apaga a data do último dia líquido")
}

func TestInsertObservationsAndLatestMetrics(t *testing.T) {
	db := openTemp(t)
	ctx := t.Context()
	seedAsset(t, db, "WEGE3", domain.ClassStock)

	runID, err := db.StartRun(ctx, "fundamentus:resultado")
	require.NoError(t, err)

	old := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	recent := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	require.NoError(t, db.InsertObservations(ctx, runID, []domain.Observation{
		obs("WEGE3", "pl", ptr(30.5), old),
		obs("WEGE3", "pl", ptr(28.1), recent),
		obs("WEGE3", "roe", nil, recent),
		obs("WEGE3", "dy", ptr(0), recent),
	}))

	a, found, err := db.GetAsset(ctx, "WEGE3")
	require.NoError(t, err)
	require.True(t, found)

	set, err := db.LatestMetrics(ctx, a.AssetID, a.Ticker)
	require.NoError(t, err)
	require.Len(t, set, 3)

	require.Equal(t, 28.1, *set["pl"].Value, "vence a observação mais recente")

	roe, ok := set["roe"]
	require.True(t, ok, "métrica coletada sem valor continua sendo chave no mapa")
	require.Nil(t, roe.Value, "ausência não vira 0")

	require.NotNil(t, set["dy"].Value)
	require.Equal(t, 0.0, *set["dy"].Value, "zero legítimo não vira ausência")

	_, ok = set["roic"]
	require.False(t, ok, "métrica nunca coletada não aparece no mapa")

	require.Equal(t, runID, set["pl"].RunID)
	require.Nil(t, set["pl"].ReferenceAt, "bulk não informa competência")
}

func TestInsertObservationsIsAppendOnly(t *testing.T) {
	db := openTemp(t)
	ctx := t.Context()
	seedAsset(t, db, "WEGE3", domain.ClassStock)

	runID, err := db.StartRun(ctx, "fundamentus:resultado")
	require.NoError(t, err)

	at := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	require.NoError(t, db.InsertObservations(ctx, runID, []domain.Observation{obs("WEGE3", "pl", ptr(28.1), at)}))
	require.Error(t, db.InsertObservations(ctx, runID, []domain.Observation{obs("WEGE3", "pl", ptr(99.9), at)}),
		"a mesma chave de proveniência colide em ux_obs")

	var stored float64
	require.NoError(t, db.QueryRow(
		`SELECT o.value FROM observation o
		 JOIN asset a ON a.asset_id = o.asset_id
		 WHERE a.ticker = 'WEGE3'`).Scan(&stored))
	require.Equal(t, 28.1, stored, "a colisão não sobrescreveu o valor")
}

func TestLatestMetricsOnEmptyDatabase(t *testing.T) {
	db := openTemp(t)
	set, err := db.LatestMetrics(t.Context(), 1, "WEGE3")
	require.NoError(t, err, "banco vazio não é erro do store")
	require.Empty(t, set)
}

func TestGetAsset(t *testing.T) {
	db := openTemp(t)
	ctx := t.Context()
	seedAsset(t, db, "MXRF11", domain.ClassFII)

	a, found, err := db.GetAsset(ctx, "MXRF11")
	require.NoError(t, err)
	require.True(t, found)
	require.Greater(t, a.AssetID, int64(0))
	require.Equal(t, domain.ClassFII, a.Class)
	require.Equal(t, "MXRF11 S.A.", a.Name)

	_, found, err = db.GetAsset(ctx, "NADA4")
	require.NoError(t, err, "ticker inexistente não é erro")
	require.False(t, found)
}

func TestUpdateAssetIdentitiesAndCoverage(t *testing.T) {
	db := openTemp(t)
	ctx := t.Context()
	seedAsset(t, db, "WEGE3", domain.ClassStock)
	seedAsset(t, db, "ITUB4", domain.ClassStock)
	seedAsset(t, db, "MXRF11", domain.ClassFII)

	total, withSector, err := db.SectorCoverage(ctx, domain.ClassStock)
	require.NoError(t, err)
	require.Equal(t, 2, total)
	require.Zero(t, withSector, "cadastro nunca rodado é zero, não ausência de ativos")

	a, _, err := db.GetAsset(ctx, "WEGE3")
	require.NoError(t, err)
	at := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	require.NoError(t, db.UpdateAssetIdentities(ctx, []AssetIdentityUpdate{{
		AssetID: a.AssetID, CNPJ: "84429695000111", ISIN: "BRWEGEACNOR0", CDCVM: "5410",
		Sector: "Bens Industriais", Subsector: "Máquinas e Equipamentos",
		Segment: "Motores. Compressores e Outros", SectorSrc: "b3", UpdatedAt: at,
	}}))

	a, _, err = db.GetAsset(ctx, "WEGE3")
	require.NoError(t, err)
	require.Equal(t, "Bens Industriais", a.Sector)
	require.Equal(t, "BRWEGEACNOR0", a.ISIN)
	require.Equal(t, "b3", a.SectorSrc)
	require.Equal(t, at, a.UpdatedAt.UTC())

	total, withSector, err = db.SectorCoverage(ctx, domain.ClassStock)
	require.NoError(t, err)
	require.Equal(t, 2, total)
	require.Equal(t, 1, withSector)
}

func TestListActiveTickers(t *testing.T) {
	db := openTemp(t)
	ctx := t.Context()
	seedAsset(t, db, "WEGE3", domain.ClassStock)
	seedAsset(t, db, "ITUB4", domain.ClassStock)
	seedAsset(t, db, "MXRF11", domain.ClassFII)

	dead, _, err := db.GetAsset(ctx, "ITUB4")
	require.NoError(t, err)
	require.NoError(t, db.UpdateAssetLiquidity(ctx, dead.AssetID, false, time.Now().UTC()))

	tickers, err := db.ListActiveTickers(ctx, domain.ClassStock)
	require.NoError(t, err)
	require.Equal(t, []string{"WEGE3"}, tickers, "inativo e outra classe ficam de fora")
}

func TestStartRunPerSource(t *testing.T) {
	db := openTemp(t)
	ctx := t.Context()

	stocks, err := db.StartRun(ctx, "fundamentus:resultado")
	require.NoError(t, err)
	fiis, err := db.StartRun(ctx, "fundamentus:fii_resultado")
	require.NoError(t, err)
	require.NotEqual(t, stocks, fiis, "um collection_run por fonte bulk")

	require.NoError(t, db.FinishRun(ctx, stocks, "ok", 900, ""))
	require.NoError(t, db.FinishRun(ctx, fiis, "failed", 0, "fonte respondeu 503"))

	var (
		status     string
		nObs       sql.NullInt64
		errMsg     sql.NullString
		finishedAt sql.NullTime
	)
	require.NoError(t, db.QueryRow(
		`SELECT status, n_obs, error, finished_at FROM collection_run WHERE id = ?`, fiis).
		Scan(&status, &nObs, &errMsg, &finishedAt))
	require.Equal(t, "failed", status)
	require.Equal(t, "fonte respondeu 503", errMsg.String)
	require.True(t, finishedAt.Valid)

	require.NoError(t, db.QueryRow(`SELECT status, error FROM collection_run WHERE id = ?`, stocks).
		Scan(&status, &errMsg))
	require.Equal(t, "ok", status)
	require.False(t, errMsg.Valid, "run sem erro grava NULL, não string vazia")
}

func TestRawDocRoundTrip(t *testing.T) {
	db := openTemp(t)
	ctx := t.Context()
	const url = "https://www.fundamentus.com.br/resultado.php"

	_, _, found, err := db.GetRawDoc(ctx, url)
	require.NoError(t, err)
	require.False(t, found)

	at := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	require.NoError(t, db.PutRawDoc(ctx, url, "universe_acao", []byte("<html>v1</html>"), at))

	body, fetchedAt, found, err := db.GetRawDoc(ctx, url)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "<html>v1</html>", string(body))
	require.Equal(t, at.UTC(), fetchedAt.UTC())

	later := at.Add(24 * time.Hour)
	require.NoError(t, db.PutRawDoc(ctx, url, "universe_acao", []byte("<html>v2</html>"), later))

	body, fetchedAt, _, err = db.GetRawDoc(ctx, url)
	require.NoError(t, err)
	require.Equal(t, "<html>v2</html>", string(body), "cache sobrescreve")
	require.Equal(t, later.UTC(), fetchedAt.UTC())

	var n int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM raw_doc`).Scan(&n))
	require.Equal(t, 1, n)
}

// Código gerado desatualizado é o modo de falha que o sqlc introduz: o SQL muda,
// o Go continua o antigo e só quebra em runtime.
func TestGeneratedCodeIsUpToDate(t *testing.T) {
	if testing.Short() {
		t.Skip("roda sqlc; fora do -short")
	}
	cmd := exec.Command("go", "tool", "sqlc", "diff")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "sqlc diff acusou divergência; rode `go generate ./internal/store/...`:\n%s", out)
}
