-- name: RecordLoginAttempt :exec
INSERT INTO login_attempts (username, ip, success)
VALUES (LOWER($1), $2, $3);

-- name: CountRecentFailures :one
SELECT count(*)::int
FROM login_attempts
WHERE ts >= now() - make_interval(mins => sqlc.arg(minutes)::int)
  AND success = false
  AND (username = LOWER(sqlc.arg(username)) OR ip = sqlc.arg(ip));

-- name: GetLastSuccessfulLoginByUsername :one
SELECT ts
FROM login_attempts
WHERE success = true AND username = LOWER($1)
ORDER BY ts DESC
LIMIT 1;
