-- Helpers to flip a user table to relational storage

BEGIN;

-- Ensure all physical columns for a table exist
CREATE OR REPLACE FUNCTION app.ensure_all_physical_columns(p_table_id bigint)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE r RECORD; BEGIN
  FOR r IN SELECT id FROM app.columns WHERE table_id = p_table_id LOOP
    PERFORM app.add_physical_column(r.id);
  END LOOP;
END; $$;

-- Cut over a table by id: ensure physical, columns, backfill, then flip storage_mode
CREATE OR REPLACE FUNCTION app.cutover_table_relational(p_table_id bigint)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE v_org uuid; BEGIN
  PERFORM app.ensure_physical_table(p_table_id);
  PERFORM app.ensure_all_physical_columns(p_table_id);
  -- backfill existing rows
  PERFORM app.backfill_physical_for_table(p_table_id);
  -- flip mode
  UPDATE app.tables SET storage_mode = 'relational', migrated_at = now() WHERE id = p_table_id;
END; $$;

-- Cut over by org + table name/slug
CREATE OR REPLACE FUNCTION app.cutover_table_relational_by_name(p_org_id uuid, p_table_name text)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE t_id bigint; BEGIN
  SELECT id INTO t_id
  FROM app.tables t
  WHERE (t.slug = lower(p_table_name) OR lower(t.name) = lower(p_table_name))
    AND t.org_id = p_org_id
  ORDER BY 1 LIMIT 1;
  IF t_id IS NULL THEN
    RAISE EXCEPTION 'Table % not found for org %', p_table_name, p_org_id;
  END IF;
  PERFORM app.cutover_table_relational(t_id);
END; $$;

COMMIT;

