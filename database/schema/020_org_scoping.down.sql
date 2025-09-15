BEGIN;

-- Roll back unique indexes
DROP INDEX IF EXISTS app_tables_org_lower_name_uniq;
DROP INDEX IF EXISTS app_tables_org_slug_uniq;

-- Attempt to restore global slug uniqueness (will fail if duplicates exist)
DO $$ BEGIN
  ALTER TABLE app.tables ADD CONSTRAINT tables_slug_key UNIQUE (slug);
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- Drop org_id column (will fail if dependencies exist)
ALTER TABLE app.tables DROP COLUMN IF EXISTS org_id;

COMMIT;
