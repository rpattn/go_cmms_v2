-- DOWN migration for seeded "Tasks" table entry and columns (fixed)
DO $$
DECLARE
  t_id        BIGINT;  -- tasks table_id in app.tables
  c_title     BIGINT;
  c_assignee  BIGINT;
  idx_name    TEXT;
BEGIN
  -- Find the logical table id for 'tasks'
  SELECT id INTO t_id FROM app.tables WHERE slug = 'tasks';

  IF t_id IS NOT NULL THEN
    -- Look up column ids
    SELECT id INTO c_title    FROM app.columns WHERE table_id = t_id AND name = 'title';
    SELECT id INTO c_assignee FROM app.columns WHERE table_id = t_id AND name = 'assignee';

    -- Drop possible ensure_index()-created index on assignee
    IF c_assignee IS NOT NULL THEN
      idx_name := format('ix_uuid_%s', c_assignee);
      EXECUTE format('DROP INDEX IF EXISTS app.%I', idx_name);
    END IF;

    -- Delete the columns (values_* rows will cascade via FK)
    DELETE FROM app.columns WHERE id IN (c_title, c_assignee);

    -- Optionally remove the table registry row if it has no data rows
    IF NOT EXISTS (SELECT 1 FROM app.rows WHERE table_id = t_id) THEN
      DELETE FROM app.tables WHERE id = t_id;
    END IF;
  END IF;
END$$;
