-- name: SearchUserTable :many
WITH params AS (
  SELECT
    sqlc.arg(table_name)::text AS table_name,
    sqlc.arg(payload)::jsonb   AS p,
    sqlc.arg(org_id)::uuid     AS org_id
),
table_id AS (
  SELECT id
  FROM app.tables t
  WHERE (t.slug = lower((SELECT table_name FROM params))
         OR lower(t.name) = lower((SELECT table_name FROM params)))
    AND (t.org_id = (SELECT org_id FROM params) OR t.org_id IS NULL)
  ORDER BY CASE WHEN t.org_id = (SELECT org_id FROM params) THEN 0 ELSE 1 END
  LIMIT 1
),
page AS (
  SELECT
    GREATEST(0, COALESCE((p->>'pageNum')::int, 0))                AS page_num,
    GREATEST(1, LEAST(COALESCE((p->>'pageSize')::int, 10), 100)) AS page_size
  FROM params
),
ff AS (
  SELECT jsonb_array_elements(p->'filterFields') AS f
  FROM params
  WHERE (p ? 'filterFields') AND jsonb_typeof(p->'filterFields') = 'array'
),
filtered AS (
  SELECT 
    b.id,
    app.row_to_json(b.id) as data,
    COUNT(*) OVER() AS total_count
  FROM app.rows b
  WHERE b.table_id = (SELECT id FROM table_id)
  AND EXISTS (
    SELECT 1
    FROM ff
    LEFT JOIN app.columns c ON c.table_id = (SELECT id FROM table_id)
      AND lower(c.name) = lower(f->>'field')
    WHERE 
      CASE
        WHEN c.type = 'text' THEN EXISTS (
          SELECT 1 FROM app.values_text vt
          WHERE vt.row_id = b.id AND vt.column_id = c.id AND (
            CASE COALESCE(f->>'operation','eq')
              WHEN 'eq' THEN vt.value = (f->>'value')
              WHEN 'cn' THEN vt.value ILIKE '%' || (f->>'value') || '%'
              WHEN 'in' THEN vt.value = ANY(ARRAY(SELECT jsonb_array_elements_text(f->'values')))
              ELSE TRUE
            END
          )
        )
        WHEN c.type = 'enum' THEN EXISTS (
          SELECT 1 FROM app.values_enum ve
          WHERE ve.row_id = b.id AND ve.column_id = c.id AND (
            CASE COALESCE(f->>'operation','eq')
              WHEN 'eq' THEN ve.value = (f->>'value')
              WHEN 'in' THEN ve.value = ANY(ARRAY(SELECT jsonb_array_elements_text(f->'values')))
              ELSE TRUE
            END
          )
        )
        WHEN c.type = 'bool' THEN EXISTS (
          SELECT 1 FROM app.values_bool vb
          WHERE vb.row_id = b.id AND vb.column_id = c.id 
          AND vb.value IS NOT DISTINCT FROM ((f->>'value')::boolean)
        )
        ELSE TRUE
      END
  )
)
SELECT 
  f.id AS row_id,
  f.data,
  f.total_count
FROM filtered f
ORDER BY f.id DESC
LIMIT (SELECT page_size FROM page)
OFFSET (SELECT page_size * page_num FROM page);

-- name: GetUserTableSchema :many
WITH params AS (
  SELECT
    sqlc.arg(table_name)::text AS table_name,
    sqlc.arg(org_id)::uuid     AS org_id
),
table_id AS (
  SELECT id
  FROM app.tables t
  WHERE (t.slug = lower((SELECT table_name FROM params))
         OR lower(t.name) = lower((SELECT table_name FROM params)))
    AND (t.org_id = (SELECT org_id FROM params) OR t.org_id IS NULL)
  ORDER BY CASE WHEN t.org_id = (SELECT org_id FROM params) THEN 0 ELSE 1 END
  LIMIT 1
)
SELECT 
  c.id,
  c.name,
  c.type::text AS type,
  c.is_required,
  c.is_indexed,
  to_jsonb(c.enum_values) AS enum_values,
  c.is_reference,
  c.reference_table_id,
  c.require_different_table
FROM app.columns c
WHERE c.table_id = (SELECT id FROM table_id)
ORDER BY c.id ASC;

-- name: CreateUserTable :one
WITH s AS (
  SELECT trim(both '-' from regexp_replace(lower(sqlc.arg(name)::text), '[^a-z0-9]+', '-', 'g')) AS slug
), ins AS (
  INSERT INTO app.tables (org_id, name, slug)
  SELECT sqlc.arg(org_id)::uuid, sqlc.arg(name)::text, s.slug FROM s
  ON CONFLICT (org_id, slug) DO NOTHING
  RETURNING id, name, slug, created_at
)
SELECT true AS created, id, name, slug, created_at FROM ins
UNION ALL
SELECT false AS created, t.id, t.name, t.slug, t.created_at
FROM app.tables t
JOIN s ON s.slug = t.slug
WHERE t.org_id = sqlc.arg(org_id)::uuid
LIMIT 1;

-- name: ListUserTables :many
SELECT id, name, slug, created_at
FROM app.tables
WHERE org_id = sqlc.arg(org_id)::uuid
ORDER BY created_at DESC, id DESC;

-- name: AddUserTableColumn :one
WITH params AS (
  SELECT
    sqlc.arg(org_id)::uuid     AS org_id,
    sqlc.arg(table_name)::text AS table_name,
    sqlc.arg(column_name)::text AS column_name,
    sqlc.arg(col_type)::text   AS col_type,
    sqlc.arg(is_required)::boolean AS is_required,
    sqlc.arg(is_indexed)::boolean  AS is_indexed,
    sqlc.arg(enum_values)::jsonb   AS enum_values,
    sqlc.arg(is_reference)::boolean AS is_reference,
    sqlc.arg(reference_table)::text AS reference_table,
    sqlc.arg(require_different_table)::boolean AS require_different_table
),
table_id AS (
  SELECT id
  FROM app.tables t
  WHERE (t.slug = lower((SELECT table_name FROM params))
         OR lower(t.name) = lower((SELECT table_name FROM params)))
    AND (t.org_id = (SELECT org_id FROM params) OR t.org_id IS NULL)
  ORDER BY CASE WHEN t.org_id = (SELECT org_id FROM params) THEN 0 ELSE 1 END
  LIMIT 1
),
cname AS (
  SELECT trim(both '_' from regexp_replace(lower((SELECT column_name FROM params)), '[^a-z0-9_]+', '_', 'g')) AS name
),
refname AS (
  SELECT NULLIF((SELECT reference_table FROM params), '') AS ref
),
ref_table_id AS (
  SELECT (
    SELECT id
    FROM app.tables t
    WHERE (t.slug = lower((SELECT ref FROM refname))
           OR lower(t.name) = lower((SELECT ref FROM refname)))
      AND (t.org_id = (SELECT org_id FROM params) OR t.org_id IS NULL)
    ORDER BY CASE WHEN t.org_id = (SELECT org_id FROM params) THEN 0 ELSE 1 END
    LIMIT 1
  ) AS id
),
ins AS (
  INSERT INTO app.columns (
    table_id, name, type, is_required, is_indexed, enum_values, is_reference, reference_table_id, require_different_table
  )
  SELECT 
    (SELECT id FROM table_id),
    (SELECT name FROM cname),
    (SELECT col_type::app.column_type FROM params),
    (SELECT is_required FROM params),
    (SELECT is_indexed FROM params),
    (
      CASE WHEN (SELECT col_type FROM params) = 'enum' THEN
        ARRAY(SELECT jsonb_array_elements_text((SELECT enum_values FROM params)))
      ELSE NULL::text[] END
    ),
    (SELECT is_reference FROM params),
    (SELECT id FROM ref_table_id),
    (SELECT require_different_table FROM params)
  ON CONFLICT (table_id, name) DO NOTHING
  RETURNING id, table_id, name, type::text AS type, is_required, is_indexed, enum_values, is_reference, reference_table_id, require_different_table
),
_ensure AS (
  SELECT CASE WHEN (SELECT is_indexed FROM params) THEN app.ensure_index(id) END FROM ins
)
SELECT true AS created,
       id, table_id, name, type, is_required, is_indexed, to_jsonb(enum_values) AS enum_values,
       is_reference, reference_table_id, require_different_table
FROM ins
UNION ALL
SELECT false AS created,
       c.id, c.table_id, c.name, c.type::text AS type, c.is_required, c.is_indexed, to_jsonb(c.enum_values) AS enum_values,
       c.is_reference, c.reference_table_id, c.require_different_table
FROM app.columns c, cname
WHERE c.table_id = (SELECT id FROM table_id) AND c.name = (SELECT name FROM cname)
LIMIT 1;

-- name: InsertUserTableRow :one
WITH params AS (
  SELECT
    sqlc.arg(org_id)::uuid     AS org_id,
    sqlc.arg(table_name)::text AS table_name,
    sqlc.arg(values)::jsonb    AS values
),
table_id AS (
  SELECT id
  FROM app.tables t
  WHERE (t.slug = lower((SELECT table_name FROM params))
         OR lower(t.name) = lower((SELECT table_name FROM params)))
    AND (t.org_id = (SELECT org_id FROM params) OR t.org_id IS NULL)
  ORDER BY CASE WHEN t.org_id = (SELECT org_id FROM params) THEN 0 ELSE 1 END
  LIMIT 1
),
ins AS (
  SELECT app.insert_row((SELECT id FROM table_id), (SELECT values FROM params)) AS row_id
)
SELECT i.row_id,
       app.row_to_json(i.row_id) AS data
FROM ins i;
