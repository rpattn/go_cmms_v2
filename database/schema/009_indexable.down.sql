-- DOWN migration for app.ensure_index(), app.on_columns_change(), and trigger

-- Drop trigger first (depends on app.on_columns_change)
DROP TRIGGER IF EXISTS trg_columns_index ON app.columns;

-- Drop the on-change trigger function
DROP FUNCTION IF EXISTS app.on_columns_change() CASCADE;

-- Drop the index helper function
DROP FUNCTION IF EXISTS app.ensure_index(bigint) CASCADE;
