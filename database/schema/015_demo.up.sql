BEGIN;

WITH p AS (
  INSERT INTO app.tables (name, slug)
  VALUES ('People','people')
  ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name
  RETURNING id
), t AS (
  INSERT INTO app.tables (name, slug)
  VALUES ('Tasks','tasks')
  ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name
  RETURNING id
)
INSERT INTO app.columns
  (table_id, name,       type,                     is_required, is_indexed, is_reference, reference_table_id, require_different_table, enum_values)
SELECT p.id, 'name',     'text'::app.column_type,  true,        true,       false,        NULL::bigint,       false,                   NULL::text[] FROM p
UNION ALL
SELECT p.id, 'birthday', 'date'::app.column_type,  false,       false,      false,        NULL::bigint,       false,                   NULL::text[] FROM p
UNION ALL
SELECT p.id, 'is_active','bool'::app.column_type,  false,       true,       false,        NULL::bigint,       false,                   NULL::text[] FROM p
UNION ALL
-- NOTE: enum_values are JSON-encoded strings: '"lead"' etc.
SELECT p.id, 'status',   'enum'::app.column_type,  false,       true,       false,        NULL::bigint,       false,                   ARRAY['lead','customer','vip']::text[] FROM p
UNION ALL
SELECT t.id, 'title',    'text'::app.column_type,  true,        false,      false,        NULL::bigint,       false,                   NULL::text[] FROM t
UNION ALL
SELECT t.id, 'assignee', 'uuid'::app.column_type,  false,       false,      true,         p.id,               true,                    NULL::text[] FROM t CROSS JOIN p
ON CONFLICT (table_id, name) DO NOTHING;

-- Insert a person (status is JSON string "vip", which now matches enum_values)
WITH people AS (SELECT id FROM app.tables WHERE slug = 'people')
SELECT app.insert_row(
  (SELECT id FROM people),
  jsonb_build_object(
    'name','Ada Lovelace',
    'birthday','1815-12-10',
    'is_active',true,
    'status','vip'
  )
) AS person_uuid;

COMMIT;
