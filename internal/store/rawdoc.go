package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

func (db *DB) GetRawDoc(ctx context.Context, url string) (body []byte, fetchedAt time.Time, found bool, err error) {
	err = db.QueryRowContext(ctx,
		`SELECT body, fetched_at FROM raw_doc WHERE url = ?`, url).Scan(&body, &fetchedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, time.Time{}, false, nil
	}
	if err != nil {
		return nil, time.Time{}, false, fmt.Errorf("get raw doc %s: %w", url, err)
	}
	return body, fetchedAt, true, nil
}

// PutRawDoc sobrescreve a entrada existente: raw_doc é cache, não fato
// histórico.
func (db *DB) PutRawDoc(ctx context.Context, url, docKind string, body []byte, fetchedAt time.Time) error {
	sum := sha256.Sum256(body)
	_, err := db.ExecContext(ctx, `
		INSERT INTO raw_doc (url, doc_kind, fetched_at, sha256, body)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(url) DO UPDATE SET
			doc_kind   = excluded.doc_kind,
			fetched_at = excluded.fetched_at,
			sha256     = excluded.sha256,
			body       = excluded.body`,
		url, docKind, fetchedAt, hex.EncodeToString(sum[:]), body)
	if err != nil {
		return fmt.Errorf("put raw doc %s: %w", url, err)
	}
	return nil
}
