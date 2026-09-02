-- name: StartRun :one
INSERT INTO collection_run (source, started_at, status)
VALUES (?, ?, 'running')
RETURNING id;

-- name: FinishRun :exec
UPDATE collection_run
SET finished_at = ?, status = ?, n_obs = ?, error = ?
WHERE id = ?;
