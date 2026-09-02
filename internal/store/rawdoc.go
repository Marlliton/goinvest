package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/marlliton/goinvest/internal/store/gen"
)

func (db *DB) GetRawDoc(ctx context.Context, url string) (body []byte, fetchedAt time.Time, found bool, err error) {
	row, err := db.q.GetRawDoc(ctx, url)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, time.Time{}, false, nil
	}
	if err != nil {
		return nil, time.Time{}, false, fmt.Errorf("get raw doc %s: %w", url, err)
	}
	return row.Body, row.FetchedAt, true, nil
}

// PutRawDoc sobrescreve a entrada existente: raw_doc é cache, não fato
// histórico.
func (db *DB) PutRawDoc(ctx context.Context, url, docKind string, body []byte, fetchedAt time.Time) error {
	sum := sha256.Sum256(body)
	err := db.q.PutRawDoc(ctx, gen.PutRawDocParams{
		Url:       url,
		DocKind:   docKind,
		FetchedAt: fetchedAt,
		Sha256:    hex.EncodeToString(sum[:]),
		Body:      body,
	})
	if err != nil {
		return fmt.Errorf("put raw doc %s: %w", url, err)
	}
	return nil
}
