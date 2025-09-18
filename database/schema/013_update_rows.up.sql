CREATE OR REPLACE FUNCTION app.update_row(p_row_id uuid, p_values jsonb)
RETURNS void LANGUAGE plpgsql AS $$
DECLARE
  t_id bigint;
  org uuid;
  rec record;
  col app.columns;
BEGIN
  SELECT table_id INTO t_id FROM app.rows WHERE id = p_row_id;
  IF t_id IS NULL THEN RAISE EXCEPTION 'Unknown row_id %', p_row_id; END IF;

  FOR rec IN SELECT key AS col_name, value FROM jsonb_each(p_values)
  LOOP
    SELECT * INTO col FROM app.columns WHERE table_id = t_id AND name = rec.col_name;
    IF col.id IS NULL THEN
      RAISE EXCEPTION 'Unknown column "%" for table_id %', rec.col_name, t_id;
    END IF;

    -- Ensure any requested index exists before we start modifying the value tables
    -- so we don't try to build an index while holding conflicting row locks.
    IF col.is_indexed THEN
      PERFORM app.ensure_index(col.id);
    END IF;

    -- Upsert into the right value table
    IF col.type='text' THEN
      INSERT INTO app.values_text(row_id, column_id, value)
      VALUES (p_row_id, col.id, rec.value::text)
      ON CONFLICT (row_id, column_id) DO UPDATE SET value = EXCLUDED.value;

    ELSIF col.type='date' THEN
      INSERT INTO app.values_date(row_id, column_id, value)
      VALUES (p_row_id, col.id, (rec.value)::date)
      ON CONFLICT (row_id, column_id) DO UPDATE SET value = EXCLUDED.value;

    ELSIF col.type='bool' THEN
      INSERT INTO app.values_bool(row_id, column_id, value)
      VALUES (p_row_id, col.id, (rec.value)::boolean)
      ON CONFLICT (row_id, column_id) DO UPDATE SET value = EXCLUDED.value;

    ELSIF col.type='enum' THEN
      INSERT INTO app.values_enum(row_id, column_id, value)
      VALUES (p_row_id, col.id, rec.value::text)
      ON CONFLICT (row_id, column_id) DO UPDATE SET value = EXCLUDED.value;

    ELSIF col.type='uuid' THEN
      INSERT INTO app.values_uuid(row_id, column_id, value)
      VALUES (p_row_id, col.id, (rec.value)::uuid)
      ON CONFLICT (row_id, column_id) DO UPDATE SET value = EXCLUDED.value;
    END IF;

  END LOOP;

  -- Dual-write: upsert into physical table with merged values
  -- Compute org_id and write the current row state to the physical table
  SELECT t.org_id INTO org FROM app.rows r JOIN app.tables t ON t.id = r.table_id WHERE r.id = p_row_id;
  IF org IS NOT NULL THEN
    PERFORM app.ensure_physical_table(t_id);
    PERFORM app.insert_row_physical(t_id, org, p_row_id, app.row_to_json(p_row_id));
  END IF;
END$$;
