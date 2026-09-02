-- name: GetRawDoc :one
SELECT body, fetched_at FROM raw_doc WHERE url = ?;

-- name: PutRawDoc :exec
INSERT INTO raw_doc (url, doc_kind, fetched_at, sha256, body)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(url) DO UPDATE SET
  doc_kind   = excluded.doc_kind,
  fetched_at = excluded.fetched_at,
  sha256     = excluded.sha256,
  body       = excluded.body;
