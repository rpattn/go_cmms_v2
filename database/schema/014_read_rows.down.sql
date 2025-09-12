-- DOWN migration for app.update_row(p_table_id bigint, p_values jsonb)

-- Just drop the function by exact signature.
DROP FUNCTION IF EXISTS app.read_rows(p_table_id bigint) CASCADE;
