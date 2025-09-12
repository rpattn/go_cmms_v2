-- DOWN migration for seeded "People" table entry and columns (fixed)
DO $$
DECLARE
  t_id        BIGINT;
  c_name      BIGINT;
  c_birthday  BIGINT;
  c_is_active BIGINT;
  c_status    BIGINT;
  idx_name    TEXT;
BEGIN
  SELECT id INTO t_id FROM app.tables WHERE slug = 'people';

  IF t_id IS NOT NULL THEN
    -- Look up column ids
    SELECT id INTO c_name      FROM app.columns WHERE table_id = t_id AND name = 'name';
    SELECT id INTO c_birthday  FROM app.columns WHERE table_id = t_id AND name = 'birthday';
    SELECT id INTO c_is_active FROM app.columns WHERE table_id = t_id AND name = 'is_active';
    SELECT id INTO c_status    FROM app.columns WHERE table_id = t_id AND name = 'status';

    -- Drop possible ensure_index()-created indexes
    IF c_name IS NOT NULL THEN
      idx_name := format('ix_text_%s', c_name);
      EXECUTE format('DROP INDEX IF EXISTS app.%I', idx_name);
    END IF;

    IF c_birthday IS NOT NULL THEN
      idx_name := format('ix_date_%s', c_birthday);
      EXECUTE format('DROP INDEX IF EXISTS app.%I', idx_name);
    END IF;

    IF c_is_active IS NOT NULL THEN
      idx_name := format('ix_bool_%s', c_is_active);
      EXECUTE format('DROP INDEX IF EXISTS app.%I', idx_name);
    END IF;

    IF c_status IS NOT NULL THEN
      idx_name := format('ix_enum_%s', c_status);
      EXECUTE format('DROP INDEX IF EXISTS app.%I', idx_name);
    END IF;

    -- Remove the columns
    DELETE FROM app.columns WHERE id IN (c_name, c_birthday, c_is_active, c_status);

    -- Optionally remove the table registry row if it has no data rows
    IF NOT EXISTS (SELECT 1 FROM app.rows WHERE table_id = t_id) THEN
      DELETE FROM app.tables WHERE id = t_id;
    END IF;
  END IF;
END$$;
