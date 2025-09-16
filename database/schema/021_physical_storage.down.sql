-- Rollback for 021_physical_storage.up.sql

BEGIN;

-- Drop helpers
DROP FUNCTION IF EXISTS app.add_physical_column(bigint);
DROP FUNCTION IF EXISTS app.ensure_physical_table(bigint);
DROP FUNCTION IF EXISTS app.physical_table_ident(bigint);

-- Drop added metadata columns (data loss warning for physical names)
ALTER TABLE IF EXISTS app.columns
  DROP COLUMN IF EXISTS enum_type_name,
  DROP COLUMN IF EXISTS physical_column_name;

ALTER TABLE IF EXISTS app.tables
  DROP COLUMN IF EXISTS migrated_at,
  DROP COLUMN IF EXISTS storage_mode,
  DROP COLUMN IF EXISTS physical_table_name,
  DROP COLUMN IF EXISTS schema_name;

-- Drop enum type
DO $$ BEGIN
  DROP TYPE IF EXISTS app.storage_mode;
EXCEPTION WHEN undefined_object THEN NULL; END $$;

-- Keep app_data schema (may contain user data); comment out next line to preserve
-- DROP SCHEMA IF EXISTS app_data CASCADE;

COMMIT;

