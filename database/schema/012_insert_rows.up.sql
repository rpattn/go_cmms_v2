CREATE OR REPLACE FUNCTION app.insert_row(p_table_id bigint, p_values jsonb)
RETURNS uuid
LANGUAGE plpgsql
AS $$
DECLARE
  r_id uuid;
  rec record;
  col app.columns;
  val_text text;  -- unwrapped scalar from jsonb (NULL if JSON null)
BEGIN
  INSERT INTO app.rows(table_id)
  VALUES (p_table_id)
  RETURNING id INTO r_id;

  -- For each key in p_values, route to the right values_* table
  FOR rec IN
    SELECT key AS col_name, value
    FROM jsonb_each(p_values)
  LOOP
    SELECT *
      INTO col
    FROM app.columns
    WHERE table_id = p_table_id
      AND name      = rec.col_name;

    IF col.id IS NULL THEN
      RAISE EXCEPTION 'Unknown column "%" for table_id %', rec.col_name, p_table_id;
    END IF;

    -- Unwrap JSON scalar to text once (JSON null -> NULL)
    val_text := rec.value #>> '{}';

    -- Required check for an explicitly provided NULL (JSON null)
    IF col.is_required AND val_text IS NULL THEN
      RAISE EXCEPTION 'Required column "%" cannot be null', col.name;
    END IF;

    -- Ensure index before we start mutating value tables so CREATE INDEX
    -- doesn't contend with the row-level locks we acquire below.
    IF col.is_indexed THEN
      PERFORM app.ensure_index(col.id);
    END IF;

    -- Type-directed insert (cast from text)
    IF col.type = 'text'::app.column_type THEN
      -- val_text is already text (can be empty string if user passed "")
      INSERT INTO app.values_text(row_id, column_id, value)
      VALUES (r_id, col.id, val_text);

    ELSIF col.type = 'date'::app.column_type THEN
      INSERT INTO app.values_date(row_id, column_id, value)
      VALUES (r_id, col.id, val_text::date);

    ELSIF col.type = 'bool'::app.column_type THEN
      INSERT INTO app.values_bool(row_id, column_id, value)
      VALUES (r_id, col.id, val_text::boolean);
    
    ELSIF col.type = 'float'::app.column_type THEN
      INSERT INTO app.values_float(row_id, column_id, value)
      VALUES (r_id, col.id, val_text::float);

    ELSIF col.type = 'enum'::app.column_type THEN
      -- Store plain label (e.g., vip). Your enum validator should compare to col.enum_values text[]
      INSERT INTO app.values_enum(row_id, column_id, value)
      VALUES (r_id, col.id, val_text);

    ELSIF col.type = 'uuid'::app.column_type THEN
      INSERT INTO app.values_uuid(row_id, column_id, value)
      VALUES (r_id, col.id, val_text::uuid);

    ELSE
      RAISE EXCEPTION 'Unsupported column type "%" for column "%"', col.type, col.name;
    END IF;

  END LOOP;

  -- Final pass: verify all required columns are present
  PERFORM 1
  FROM app.columns c
  WHERE c.table_id = p_table_id
    AND c.is_required
    AND NOT EXISTS (
      SELECT 1 FROM app.values_text vt WHERE vt.row_id = r_id AND vt.column_id = c.id
      UNION ALL
      SELECT 1 FROM app.values_float vf WHERE vf.row_id = r_id AND vf.column_id = c.id
      UNION ALL
      SELECT 1 FROM app.values_date vd WHERE vd.row_id = r_id AND vd.column_id = c.id
      UNION ALL
      SELECT 1 FROM app.values_bool vb WHERE vb.row_id = r_id AND vb.column_id = c.id
      UNION ALL
      SELECT 1 FROM app.values_enum ve WHERE ve.row_id = r_id AND ve.column_id = c.id
      UNION ALL
      SELECT 1 FROM app.values_uuid vu WHERE vu.row_id = r_id AND vu.column_id = c.id
    );

  IF FOUND THEN
    RAISE EXCEPTION 'Missing required columns for table_id %', p_table_id;
  END IF;

  RETURN r_id;
END
$$;
