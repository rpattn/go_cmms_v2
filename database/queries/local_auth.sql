-- name: CreateLocalCredential :exec
INSERT INTO local_credentials (user_id, username, password_hash)
VALUES ($1, LOWER($2), $3);

-- name: GetLocalCredentialByUsername :one
SELECT
  lc.user_id,
  lc.username,
  lc.password_hash,
  u.email,
  u.name
FROM local_credentials lc
JOIN users u ON u.id = lc.user_id
WHERE lc.username = LOWER($1);

-- name: UpdateLocalPasswordHash :exec
UPDATE local_credentials
SET password_hash = $2,
    last_password_change = now()
WHERE user_id = $1;
