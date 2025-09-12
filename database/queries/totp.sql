-- name: SetTOTPSecret :exec
INSERT INTO user_totp (user_id, secret, issuer, label)
VALUES ($1, $2, $3, $4)
ON CONFLICT (user_id)
DO UPDATE SET secret = EXCLUDED.secret,
              issuer = EXCLUDED.issuer,
              label  = EXCLUDED.label;

-- name: GetTOTPSecret :one
SELECT secret
FROM user_totp
WHERE user_id = $1;

-- name: UserHasTOTP :one
SELECT EXISTS (
  SELECT 1 FROM user_totp WHERE user_id = $1
) AS has_totp;
