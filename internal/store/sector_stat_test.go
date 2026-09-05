package store

import (
	"fmt"
	"testing"
	"time"

	"github.com/marlliton/goinvest/internal/domain"
	"github.com/stretchr/testify/require"
)

var computedAt = time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)

func seedGraded(t *testing.T, db *DB, ticker string, class domain.AssetClass, sector, subsector, segment string, values map[domain.MetricID]float64) {
	t.Helper()
	ctx := t.Context()

	require.NoError(t, db.UpsertAsset(ctx, ticker, class, ticker, computedAt))
	a, found, err := db.GetAsset(ctx, ticker)
	require.NoError(t, err)
	require.True(t, found)
	require.NoError(t, db.UpdateAssetLiquidity(ctx, a.AssetID, true, computedAt))

	if sector != "" {
		require.NoError(t, db.UpdateAssetIdentities(ctx, []AssetIdentityUpdate{{
			AssetID: a.AssetID, Sector: sector, Subsector: subsector, Segment: segment,
			SectorSrc: "b3", UpdatedAt: computedAt,
		}}))
	}

	if len(values) == 0 {
		return
	}
	runID, err := db.StartRun(ctx, "test")
	require.NoError(t, err)
	obs := make([]domain.Observation, 0, len(values))
	for id, v := range values {
		obs = append(obs, domain.Observation{
			Ticker: ticker, Metric: id, PeriodKind: "spot", PeriodEnd: computedAt,
			Value: &v, Unit: domain.UnitRatio, Source: "test", FetchedAt: computedAt,
		})
	}
	require.NoError(t, db.InsertObservations(ctx, runID, obs))
}

func plRules() []MetricRule {
	return []MetricRule{{MetricID: "pl"}}
}

func assetOf(t *testing.T, db *DB, ticker string) domain.Asset {
	t.Helper()
	a, found, err := db.GetAsset(t.Context(), ticker)
	require.NoError(t, err)
	require.True(t, found, ticker)
	return a
}

func TestRecomputeSectorStatsCascade(t *testing.T) {
	db := openTemp(t)

	for i, ticker := range []string{"AAAA3", "BBBB3", "CCCC3", "DDDD3", "EEEE3"} {
		seedGraded(t, db, ticker, domain.ClassStock, "Bens Industriais", "Máquinas", "Motores",
			map[domain.MetricID]float64{"pl": float64(10 + i)})
	}
	// Setor inteiro com 4 papéis: nem o segmento, nem o subsetor, nem o setor
	// alcançam o piso, então a cascata desce até o mercado.
	for i, ticker := range []string{"FFFF3", "GGGG3", "HHHH3", "IIII3"} {
		seedGraded(t, db, ticker, domain.ClassStock, "Comunicações", "Telecom", "Telefonia",
			map[domain.MetricID]float64{"pl": float64(20 + i)})
	}
	seedGraded(t, db, "ZZZZ3", domain.ClassStock, "", "", "",
		map[domain.MetricID]float64{"pl": 5})

	require.NoError(t, db.RecomputeSectorStats(t.Context(), plRules(), computedAt))

	a := assetOf(t, db, "AAAA3")
	require.Equal(t, "segmento", a.PeerGroupLevel)
	require.Equal(t, "Motores", a.PeerGroupKey)
	require.NotNil(t, a.PeerGroupN)
	require.Equal(t, 5, *a.PeerGroupN)

	f := assetOf(t, db, "FFFF3")
	require.Equal(t, "mercado", f.PeerGroupLevel)
	require.Equal(t, 9, *f.PeerGroupN, "o mercado da classe inclui os dois setores")

	z := assetOf(t, db, "ZZZZ3")
	require.Empty(t, z.PeerGroupLevel, "sem setor é categoria própria, não mercado por omissão")
	require.Nil(t, z.PeerGroupN)
}

func TestRecomputeSectorStatsIsIdempotent(t *testing.T) {
	db := openTemp(t)
	for i, ticker := range []string{"AAAA3", "BBBB3", "CCCC3", "DDDD3", "EEEE3"} {
		seedGraded(t, db, ticker, domain.ClassStock, "Bens Industriais", "Máquinas", "Motores",
			map[domain.MetricID]float64{"pl": float64(10 + i)})
	}

	require.NoError(t, db.RecomputeSectorStats(t.Context(), plRules(), computedAt))
	first := countRows(t, db)

	require.NoError(t, db.RecomputeSectorStats(t.Context(), plRules(), computedAt))
	require.Equal(t, first, countRows(t, db))
}

func countRows(t *testing.T, db *DB) [2]int {
	t.Helper()
	var stats, pct int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM sector_stat`).Scan(&stats))
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM asset_percentile`).Scan(&pct))
	return [2]int{stats, pct}
}

func TestRecomputeSectorStatsExcludesNegativeWhenDeclared(t *testing.T) {
	db := openTemp(t)
	values := []float64{10, 12, 14, 16, 18, -5}
	for i, ticker := range []string{"AAAA3", "BBBB3", "CCCC3", "DDDD3", "EEEE3", "NEGA3"} {
		seedGraded(t, db, ticker, domain.ClassStock, "Bens Industriais", "Máquinas", "Motores",
			map[domain.MetricID]float64{"pl": values[i]})
	}

	require.NoError(t, db.RecomputeSectorStats(t.Context(),
		[]MetricRule{{MetricID: "pl", ExcludeNegative: true}}, computedAt))

	neg := assetOf(t, db, "NEGA3")
	pcts, err := db.GetAssetPercentiles(t.Context(), neg.AssetID)
	require.NoError(t, err)
	require.Empty(t, pcts, "P/L negativo não recebe posição na distribuição")

	ok := assetOf(t, db, "AAAA3")
	pcts, err = db.GetAssetPercentiles(t.Context(), ok.AssetID)
	require.NoError(t, err)
	require.Len(t, pcts, 1)
	require.Equal(t, 5, pcts[0].N, "o negativo saiu da população")
}

func TestRecomputeSectorStatsExcludesSentinelBySegment(t *testing.T) {
	db := openTemp(t)
	values := []float64{10, 12, 14, 16, 18, 0}
	for i, ticker := range []string{"AAAA3", "BBBB3", "CCCC3", "DDDD3", "EEEE3", "BANK3"} {
		seedGraded(t, db, ticker, domain.ClassStock, "Financeiro", "Intermediários", "Bancos",
			map[domain.MetricID]float64{"ev_ebitda": values[i]})
	}

	require.NoError(t, db.RecomputeSectorStats(t.Context(),
		[]MetricRule{{MetricID: "ev_ebitda", SentinelSegments: []string{"Bancos"}}}, computedAt))

	bank := assetOf(t, db, "BANK3")
	pcts, err := db.GetAssetPercentiles(t.Context(), bank.AssetID)
	require.NoError(t, err)
	require.Empty(t, pcts, "0,00 em banco é código de ausência, não valor")

	ok := assetOf(t, db, "AAAA3")
	pcts, err = db.GetAssetPercentiles(t.Context(), ok.AssetID)
	require.NoError(t, err)
	require.Equal(t, 5, pcts[0].N)
}

func TestRecomputeSectorStatsSectorGroupIncludesAllLiquidMembers(t *testing.T) {
	db := openTemp(t)

	for i := 0; i < 11; i++ {
		ticker := fmt.Sprintf("SEG%02d3", i)
		seedGraded(t, db, ticker, domain.ClassStock, "Bens Industriais", "Máquinas", "Motores",
			map[domain.MetricID]float64{"pl": float64(10 + i)})
	}
	for i := 0; i < 5; i++ {
		ticker := fmt.Sprintf("RES%02d3", i)
		seedGraded(t, db, ticker, domain.ClassStock, "Bens Industriais", "", "",
			map[domain.MetricID]float64{"pl": float64(100 + i)})
	}

	require.NoError(t, db.RecomputeSectorStats(t.Context(), plRules(), computedAt))

	res := assetOf(t, db, "RES003")
	require.Equal(t, "setor", res.PeerGroupLevel)
	require.Equal(t, "Bens Industriais", res.PeerGroupKey)

	pcts, err := db.GetAssetPercentiles(t.Context(), res.AssetID)
	require.NoError(t, err)
	require.Len(t, pcts, 1)
	require.Equal(t, 16, pcts[0].N,
		"o grupo do setor deve conter os 16 líquidos, inclusive quem resolveu em segmento")

	var setorN int64
	require.NoError(t, db.QueryRow(
		`SELECT n FROM sector_stat WHERE group_level = 'setor' AND group_key = 'Bens Industriais' AND metric_id = 'pl'`,
	).Scan(&setorN))
	require.Equal(t, int64(16), setorN,
		"sector_stat do setor não pode ficar truncado nos 5 que resolveram ali")

	var subsetorN int64
	require.NoError(t, db.QueryRow(
		`SELECT n FROM sector_stat WHERE group_level = 'subsetor' AND group_key = 'Máquinas' AND metric_id = 'pl'`,
	).Scan(&subsetorN))
	require.Equal(t, int64(11), subsetorN,
		"subsetor precisa existir com o n do próprio nível mesmo quando ninguém resolveu ali")
}

func TestRecomputeSectorStatsFallsBackPerMetricWhenSparse(t *testing.T) {
	db := openTemp(t)

	seedGraded(t, db, "AAAA3", domain.ClassStock, "Bens Industriais", "Máquinas", "Motores",
		map[domain.MetricID]float64{"pl": 10})
	for _, ticker := range []string{"BBBB3", "CCCC3", "DDDD3", "EEEE3"} {
		seedGraded(t, db, ticker, domain.ClassStock, "Bens Industriais", "Máquinas", "Motores", nil)
	}
	for i, ticker := range []string{"FFFF3", "GGGG3", "HHHH3", "IIII3", "JJJJ3"} {
		seedGraded(t, db, ticker, domain.ClassStock, "Financeiro", "Bancos", "Bancos",
			map[domain.MetricID]float64{"pl": float64(20 + i)})
	}

	require.NoError(t, db.RecomputeSectorStats(t.Context(), plRules(), computedAt))

	a := assetOf(t, db, "AAAA3")
	require.Equal(t, "segmento", a.PeerGroupLevel, "o grupo do cabeçalho não muda")

	pcts, err := db.GetAssetPercentiles(t.Context(), a.AssetID)
	require.NoError(t, err)
	require.Len(t, pcts, 1)
	require.True(t, pcts[0].FellBackToMarket, "só um peer tem P/L: a linha compara contra o mercado")
	require.Equal(t, 6, pcts[0].N)
}
