package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGetSelicNeverCollected(t *testing.T) {
	db := openTemp(t)

	value, referenceAt, fetchedAt, found, err := db.GetSelic(t.Context())
	require.NoError(t, err)
	require.False(t, found)
	require.Nil(t, value)
	require.Nil(t, referenceAt)
	require.Nil(t, fetchedAt)
}

func TestPutSelicOverwritesSingleRow(t *testing.T) {
	db := openTemp(t)
	ref := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	fetched := time.Date(2026, 9, 17, 10, 30, 0, 0, time.UTC)

	require.NoError(t, db.PutSelic(t.Context(), 0.14, ref, fetched))

	value, referenceAt, fetchedAt, found, err := db.GetSelic(t.Context())
	require.NoError(t, err)
	require.True(t, found)
	require.InDelta(t, 0.14, *value, 1e-9)
	require.Equal(t, ref, referenceAt.UTC())
	require.Equal(t, fetched, fetchedAt.UTC())

	newRef := ref.AddDate(0, 0, 30)
	require.NoError(t, db.PutSelic(t.Context(), 0.1375, newRef, fetched.AddDate(0, 0, 30)))

	value, referenceAt, _, found, err = db.GetSelic(t.Context())
	require.NoError(t, err)
	require.True(t, found)
	require.InDelta(t, 0.1375, *value, 1e-9)
	require.Equal(t, newRef, referenceAt.UTC())

	var rows int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM macro_rate`).Scan(&rows))
	require.Equal(t, 1, rows)
}
