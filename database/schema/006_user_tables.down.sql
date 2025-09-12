-- DOWN migration for user-defined logical tables

-- Drop per-type value tables first (depend on app.rows and app.columns)
DROP TABLE IF EXISTS app.values_uuid;
DROP TABLE IF EXISTS app.values_enum;
DROP TABLE IF EXISTS app.values_bool;
DROP TABLE IF EXISTS app.values_date;
DROP TABLE IF EXISTS app.values_text;
DROP TABLE IF EXISTS app.values_float;

-- Then the row store (depends on app.tables and pgcrypto's gen_random_uuid default)
DROP TABLE IF EXISTS app.rows;

-- Then columns (depends on app.tables and app.column_type)
DROP TABLE IF EXISTS app.columns;

-- Then the enum type used by app.columns
DROP TYPE IF EXISTS app.column_type;

-- Finally the logical tables registry
DROP TABLE IF EXISTS app.tables;
