-- name: UpsertUserByVerifiedEmail :one
INSERT INTO users (email, name)
VALUES ($1, $2)
ON CONFLICT (email)
DO UPDATE SET name = COALESCE(users.name, EXCLUDED.name)
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- params: $1 = user_id (UUID), $2 = org_id (UUID)
-- name: GetUserWithOrgAndRole :one
WITH
  input AS (
    SELECT $1::uuid AS user_id, $2::uuid AS org_id
  ),
  u AS (
    SELECT u.id, u.email, u.name, u.avatar_url, u.phone, u.country, u.created_at
    FROM users u
    JOIN input i ON u.id = i.user_id
  ),
  o AS (
    SELECT o.id, o.slug, o.name, o.created_at
    FROM organisations o
    JOIN input i ON o.id = i.org_id
  ),
  m AS (
    SELECT om.org_id, om.user_id, om.role
    FROM org_memberships om
    JOIN o ON om.org_id = o.id
    JOIN u ON om.user_id = u.id
  )
SELECT
  u.id            AS user_id,
  u.email         AS user_email,
  u.name          AS user_name,
  u.avatar_url    AS user_avatar_url,
  u.phone         AS user_phone,
  u.country       AS user_country,
  o.id            AS org_id,
  o.slug          AS org_slug,
  o.name          AS org_name,
  m.role::text    AS role,
  (u.id IS NOT NULL)::bool AS user_exists,
  (o.id IS NOT NULL)::bool AS org_exists,
  (m.role IS NOT NULL)::bool AS role_exists
FROM input i
LEFT JOIN u ON TRUE
LEFT JOIN o ON TRUE
LEFT JOIN m ON TRUE
LIMIT 1;



-- name: SearchOrgUsers :many
WITH input AS (
  SELECT
    sqlc.arg(org_id)::uuid   AS org_id,
    sqlc.arg(payload)::jsonb AS payload
),
params AS (
  SELECT
    org_id,
    GREATEST(1, LEAST(COALESCE((payload->>'pageSize')::int, 10), 100)) AS page_size,
    GREATEST(0, COALESCE((payload->>'pageNum')::int, 0))                AS page_num,
    COALESCE(payload->'filterFields', '[]'::jsonb)                      AS filters
  FROM input
),
base AS (
  SELECT u.id, u.email, u.name, u.created_at
  FROM users u
  JOIN org_memberships m ON m.user_id = u.id
  WHERE m.org_id = (SELECT org_id FROM params)
),
filtered AS (
  SELECT
    b.*,
    COUNT(*) OVER () AS total_count
  FROM base b
  LEFT JOIN LATERAL (
    SELECT
      grp_idx,
      BOOL_OR(
        CASE col
          WHEN 'email' THEN
            CASE op
              WHEN 'eq' THEN b.email = val
              WHEN 'cn' THEN b.email ILIKE '%' || val || '%'
              WHEN 'sw' THEN b.email ILIKE val || '%'
              WHEN 'ew' THEN b.email ILIKE '%' || val
              ELSE b.email = val
            END
          WHEN 'name' THEN
            CASE op
              WHEN 'eq' THEN b.name = val
              WHEN 'cn' THEN b.name ILIKE '%' || val || '%'
              WHEN 'sw' THEN b.name ILIKE val || '%'
              WHEN 'ew' THEN b.name ILIKE '%' || val
              ELSE b.name = val
            END
          ELSE FALSE
        END
      ) AS group_match,
      BOOL_OR(col IS NOT NULL) AS has_recognized
    FROM (
      SELECT ROW_NUMBER() OVER () AS grp_idx, g.elem
      FROM params p,
           LATERAL jsonb_array_elements(p.filters) AS g(elem)
    ) grp
    CROSS JOIN LATERAL (
      SELECT
        CASE LOWER(COALESCE(grp.elem->>'field',''))
          WHEN 'email'     THEN 'email'
          WHEN 'name'      THEN 'name'
          WHEN 'firstname' THEN 'name'
          WHEN 'lastname'  THEN 'name'
          ELSE NULL
        END                                    AS col,
        LOWER(COALESCE(grp.elem->>'operation','eq')) AS op,
        COALESCE(grp.elem->>'value','')        AS val
      UNION ALL
      SELECT
        CASE LOWER(COALESCE(alt->>'field',''))
          WHEN 'email'     THEN 'email'
          WHEN 'name'      THEN 'name'
          WHEN 'firstname' THEN 'name'
          WHEN 'lastname'  THEN 'name'
          ELSE NULL
        END,
        LOWER(COALESCE(alt->>'operation','eq')),
        COALESCE(alt->>'value','')
      FROM jsonb_array_elements(COALESCE(grp.elem->'alternatives','[]'::jsonb)) alt
    ) c
    GROUP BY grp_idx
  ) g ON TRUE
  GROUP BY b.id, b.email, b.name, b.created_at
  HAVING COALESCE(BOOL_AND((NOT g.has_recognized) OR g.group_match), TRUE)
)
SELECT
  id::uuid       AS id,
  email::text    AS email,
  name::text     AS name,
  created_at     AS created_at,
  total_count    AS total_count
FROM filtered
ORDER BY created_at DESC
LIMIT (SELECT page_size FROM params)
OFFSET (SELECT page_size * page_num FROM params);

-- name: UpdateUserProfile :exec
UPDATE users SET
  name       = COALESCE(sqlc.narg(name), name),
  avatar_url = COALESCE(sqlc.narg(avatar_url), avatar_url),
  phone      = COALESCE(sqlc.narg(phone), phone),
  country    = COALESCE(sqlc.narg(country), country)
WHERE id = sqlc.arg(user_id)::uuid;
