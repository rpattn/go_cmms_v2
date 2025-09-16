BEGIN;

-- Roll back unique indexes
DROP INDEX IF EXISTS app_tables_org_lower_name_uniq;
DROP INDEX IF EXISTS app_tables_org_slug_uniq;

-- Attempt to restore global slug uniqueness only if no duplicates exist
DO $$
BEGIN
  IF EXISTS (
      SELECT 1 FROM (
        SELECT slug FROM app.tables GROUP BY slug HAVING COUNT(*) > 1
      ) dups
    ) THEN
    RAISE NOTICE 'Skipping restoration of global unique(slug); duplicates exist.';
  ELSE
    BEGIN
      ALTER TABLE app.tables ADD CONSTRAINT tables_slug_key UNIQUE (slug);
    EXCEPTION WHEN duplicate_object THEN
      NULL;
    WHEN unique_violation THEN
      -- Defensive: if concurrent inserts created duplicates, skip
      RAISE NOTICE 'Could not enforce unique(slug) due to duplicates; leaving unconstrained.';
    END;
  END IF;
END $$;

-- Drop org_id column (will fail if dependencies exist)
ALTER TABLE app.tables DROP COLUMN IF EXISTS org_id;

COMMIT;
