-- DOWN migration for app.insert_row(p_table_id bigint, p_values jsonb)

-- Just drop the function by exact signature.
DROP FUNCTION IF EXISTS app.insert_row(bigint, jsonb) CASCADE;
