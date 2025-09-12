-- Create Tasks table entry
WITH tasks AS (
  INSERT INTO app.tables (name, slug)
  VALUES ('Tasks', 'tasks')
  RETURNING id
)
INSERT INTO app.columns (table_id, name, type, is_required)
SELECT id, 'title', 'text', true
FROM tasks;

-- Create 'assignee' as a UUID reference to People
WITH tasks AS (
  SELECT id FROM app.tables WHERE slug = 'tasks'
),
people AS (
  SELECT id FROM app.tables WHERE slug = 'people'
)
INSERT INTO app.columns (table_id, name, type, is_reference, reference_table_id, require_different_table)
SELECT t.id, 'assignee', 'uuid', true, p.id, true
FROM tasks t, people p;
