-- Backfill helpers from EAV into physical tables

BEGIN;

-- Backfill a single table; returns rows processed
CREATE OR REPLACE FUNCTION app.backfill_physical_for_table(p_table_id bigint)
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
  v_org uuid;
  v_count bigint := 0;
  r RECORD;
BEGIN
  SELECT org_id INTO v_org FROM app.tables WHERE id = p_table_id;
  IF v_org IS NULL THEN
    RAISE EXCEPTION 'Table % has no org_id; cannot backfill', p_table_id;
  END IF;

  PERFORM app.ensure_physical_table(p_table_id);

  FOR r IN (
    SELECT id FROM app.rows WHERE table_id = p_table_id ORDER BY created_at ASC
  ) LOOP
    PERFORM app.insert_row_physical(p_table_id, v_org, r.id, app.row_to_json(r.id));
    v_count := v_count + 1;
  END LOOP;

  RETURN v_count;
END;
$$;

-- Backfill all tables; returns total rows processed
CREATE OR REPLACE FUNCTION app.backfill_physical_all()
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
  t RECORD;
  total bigint := 0;
BEGIN
  FOR t IN (
    SELECT id FROM app.tables WHERE org_id IS NOT NULL ORDER BY id
  ) LOOP
    total := total + app.backfill_physical_for_table(t.id);
  END LOOP;
  RETURN total;
END;
$$;

COMMIT;

