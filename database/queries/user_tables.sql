-- name: SearchUserTable :many
WITH params AS (
  SELECT
    sqlc.arg(table_name)::text AS table_name,
    sqlc.arg(payload)::jsonb   AS p
),
table_id AS (
  SELECT COALESCE(
    (SELECT id FROM app.tables WHERE slug = lower((SELECT table_name FROM params))),
    (SELECT id FROM app.tables WHERE lower(name) = lower((SELECT table_name FROM params)))
  ) AS id
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