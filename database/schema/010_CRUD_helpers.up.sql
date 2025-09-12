-- Create People table entry and columns (fixed types + idempotent)
WITH people AS (
  INSERT INTO app.tables (name, slug)
  VALUES ('People', 'people')
  ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name
  RETURNING id
)
INSERT INTO app.columns (table_id, name, type, is_required, is_indexed)
SELECT id, 'name', 'text'::app.column_type, true, true
FROM people
ON CONFLICT (table_id, name) DO NOTHING;

WITH people AS (
  SELECT id FROM app.tables WHERE slug = 'people'
)
INSERT INTO app.columns (table_id, name, type)
SELECT id, 'birthday', 'date'::app.column_type
FROM people
ON CONFLICT (table_id, name) DO NOTHING;

WITH people AS (
  SELECT id FROM app.tables WHERE slug = 'people'
)
INSERT INTO app.columns (table_id, name, type, is_indexed)
SELECT id, 'is_active', 'bool'::app.column_type, true
FROM people
ON CONFLICT (table_id, name) DO NOTHING;

WITH people AS (
  SELECT id FROM app.tables WHERE slug = 'people'
)
INSERT INTO app.columns (table_id, name, type, enum_values, is_indexed)
SELECT id,
       'status',
       'enum'::app.column_type,
       ARRAY['lead','customer','vip']::text[],  -- JSON-encoded labels to match current validator
       true
FROM people
ON CONFLICT (table_id, name) DO NOTHING;
