package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGetCDINeverCollected(t *testing.T) {
	db := openTemp(t)

	value, referenceAt, fetchedAt, found, err := db.GetCDI(t.Context())
	require.NoError(t, err)
	require.False(t, found)
	require.Nil(t, value)
	require.Nil(t, referenceAt)
	require.Nil(t, fetchedAt)
}

func TestPutCDIOverwritesSingleRow(t *testing.T) {
	db := openTemp(t)
	ref := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	fetched := time.Date(2026, 9, 17, 10, 30, 0, 0, time.UTC)

	require.NoError(t, db.PutCDI(t.Context(), 0.1365, ref, fetched))

	value, referenceAt, fetchedAt, found, err := db.GetCDI(t.Context())
	require.NoError(t, err)
	require.True(t, found)
	require.InDelta(t, 0.1365, *value, 1e-9)
	require.Equal(t, ref, referenceAt.UTC())
	require.Equal(t, fetched, fetchedAt.UTC())

	newRef := ref.AddDate(0, 0, 30)
	require.NoError(t, db.PutCDI(t.Context(), 0.1340, newRef, fetched.AddDate(0, 0, 30)))

	value, referenceAt, _, found, err = db.GetCDI(t.Context())
	require.NoError(t, err)
	require.True(t, found)
	require.InDelta(t, 0.1340, *value, 1e-9)
	require.Equal(t, newRef, referenceAt.UTC())

	var rows int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM macro_rate WHERE id = ?`, cdiRateID).Scan(&rows))
	require.Equal(t, 1, rows)
}

func TestGetTesouroIPCA10yNeverCollected(t *testing.T) {
	db := openTemp(t)

	value, referenceAt, fetchedAt, found, err := db.GetTesouroIPCA10y(t.Context())
	require.NoError(t, err)
	require.False(t, found)
	require.Nil(t, value)
	require.Nil(t, referenceAt)
	require.Nil(t, fetchedAt)
}

func TestPutTesouroIPCA10yOverwritesSingleRow(t *testing.T) {
	db := openTemp(t)
	ref := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	fetched := time.Date(2026, 9, 17, 10, 30, 0, 0, time.UTC)

	require.NoError(t, db.PutTesouroIPCA10y(t.Context(), 0.07, ref, fetched))

	value, referenceAt, fetchedAt, found, err := db.GetTesouroIPCA10y(t.Context())
	require.NoError(t, err)
	require.True(t, found)
	require.InDelta(t, 0.07, *value, 1e-9)
	require.Equal(t, ref, referenceAt.UTC())
	require.Equal(t, fetched, fetchedAt.UTC())

	newRef := ref.AddDate(0, 0, 30)
	require.NoError(t, db.PutTesouroIPCA10y(t.Context(), 0.0685, newRef, fetched.AddDate(0, 0, 30)))

	value, referenceAt, _, found, err = db.GetTesouroIPCA10y(t.Context())
	require.NoError(t, err)
	require.True(t, found)
	require.InDelta(t, 0.0685, *value, 1e-9)
	require.Equal(t, newRef, referenceAt.UTC())

	var rows int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM macro_rate WHERE id = ?`, tesouroIPCA10yRateID).Scan(&rows))
	require.Equal(t, 1, rows)
}

func TestGetFocusIPCA12mNeverCollected(t *testing.T) {
	db := openTemp(t)

	value, referenceAt, fetchedAt, found, err := db.GetFocusIPCA12m(t.Context())
	require.NoError(t, err)
	require.False(t, found)
	require.Nil(t, value)
	require.Nil(t, referenceAt)
	require.Nil(t, fetchedAt)
}

func TestPutFocusIPCA12mOverwritesSingleRow(t *testing.T) {
	db := openTemp(t)
	ref := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	fetched := time.Date(2026, 9, 17, 10, 30, 0, 0, time.UTC)

	require.NoError(t, db.PutFocusIPCA12m(t.Context(), 0.045, ref, fetched))

	value, referenceAt, fetchedAt, found, err := db.GetFocusIPCA12m(t.Context())
	require.NoError(t, err)
	require.True(t, found)
	require.InDelta(t, 0.045, *value, 1e-9)
	require.Equal(t, ref, referenceAt.UTC())
	require.Equal(t, fetched, fetchedAt.UTC())

	newRef := ref.AddDate(0, 0, 30)
	require.NoError(t, db.PutFocusIPCA12m(t.Context(), 0.044, newRef, fetched.AddDate(0, 0, 30)))

	value, referenceAt, _, found, err = db.GetFocusIPCA12m(t.Context())
	require.NoError(t, err)
	require.True(t, found)
	require.InDelta(t, 0.044, *value, 1e-9)
	require.Equal(t, newRef, referenceAt.UTC())

	var rows int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM macro_rate WHERE id = ?`, focusIPCA12mRateID).Scan(&rows))
	require.Equal(t, 1, rows)
}

func TestFourAnchorsCoexistInMacroRate(t *testing.T) {
	db := openTemp(t)
	ref := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	fetched := time.Date(2026, 9, 17, 10, 30, 0, 0, time.UTC)

	require.NoError(t, db.PutSelic(t.Context(), 0.1375, ref, fetched))
	require.NoError(t, db.PutCDI(t.Context(), 0.1365, ref, fetched))
	require.NoError(t, db.PutTesouroIPCA10y(t.Context(), 0.07, ref, fetched))
	require.NoError(t, db.PutFocusIPCA12m(t.Context(), 0.045, ref, fetched))

	var rows int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM macro_rate`).Scan(&rows))
	require.Equal(t, 4, rows)

	selic, _, _, found, err := db.GetSelic(t.Context())
	require.NoError(t, err)
	require.True(t, found)
	require.InDelta(t, 0.1375, *selic, 1e-9)

	cdi, _, _, found, err := db.GetCDI(t.Context())
	require.NoError(t, err)
	require.True(t, found)
	require.InDelta(t, 0.1365, *cdi, 1e-9)

	tesouroIPCA, _, _, found, err := db.GetTesouroIPCA10y(t.Context())
	require.NoError(t, err)
	require.True(t, found)
	require.InDelta(t, 0.07, *tesouroIPCA, 1e-9)

	focusIPCA, _, _, found, err := db.GetFocusIPCA12m(t.Context())
	require.NoError(t, err)
	require.True(t, found)
	require.InDelta(t, 0.045, *focusIPCA, 1e-9)
}
