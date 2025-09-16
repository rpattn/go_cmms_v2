-- Dual-write helpers for table-per-collection
-- - Adds physical insert/upsert and delete functions

BEGIN;

-- Upsert row into physical table for a logical table
CREATE OR REPLACE FUNCTION app.insert_row_physical(
  p_table_id bigint,
  p_org_id uuid,
  p_row_id uuid,
  p_values jsonb
) RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  sname text;
  tname text;
  col_list text := '';
  expr_list text := '';
  upd_list text := 'org_id = EXCLUDED.org_id, table_id = EXCLUDED.table_id';
  rec RECORD;
  first boolean := true;
  enum_t text;
BEGIN
  -- ensure table exists and resolve identifiers
  SELECT COALESCE(t.schema_name,'app_data'), COALESCE(t.physical_table_name, 't_'||p_table_id::text)
  INTO sname, tname
  FROM app.tables t
  WHERE t.id = p_table_id
  FOR UPDATE;

  PERFORM app.ensure_physical_table(p_table_id);

  -- build dynamic column and expression lists from metadata
  FOR rec IN (
    SELECT c.id AS col_id, c.name AS logical_name, COALESCE(c.physical_column_name, 'c_'||c.id::text) AS phys_name,
           c.type, COALESCE(c.enum_type_name, format('%I.%I', COALESCE(t.schema_name,'app_data'), 'e_'||c.id::text)) AS enum_type
    FROM app.columns c
    JOIN app.tables t ON t.id = c.table_id
    WHERE c.table_id = p_table_id
    ORDER BY c.id
  ) LOOP
    IF NOT first THEN
      col_list := col_list || ', ';
      expr_list := expr_list || ', ';
      upd_list := upd_list || ', ';
    END IF;
    first := false;

    col_list := col_list || format('%I', rec.phys_name);
    CASE rec.type
      WHEN 'text' THEN
        expr_list := expr_list || format('NULLIF(($4->>%L), '''')::text', rec.logical_name);
      WHEN 'date' THEN
        expr_list := expr_list || format('NULLIF(($4->>%L), '''')::date', rec.logical_name);
      WHEN 'bool' THEN
        expr_list := expr_list || format('NULLIF(($4->>%L), '''')::boolean', rec.logical_name);
      WHEN 'float' THEN
        expr_list := expr_list || format('NULLIF(($4->>%L), '''')::double precision', rec.logical_name);
      WHEN 'uuid' THEN
        expr_list := expr_list || format('NULLIF(($4->>%L), '''')::uuid', rec.logical_name);
      WHEN 'enum' THEN
        expr_list := expr_list || format('NULLIF(($4->>%L), '''')::%s', rec.logical_name, rec.enum_type);
      ELSE
        RAISE EXCEPTION 'Unsupported column type: %', rec.type;
    END CASE;
    upd_list := upd_list || format('%I = EXCLUDED.%I', rec.phys_name, rec.phys_name);
  END LOOP;

  IF col_list = '' THEN
    -- only base columns
    EXECUTE format(
      'INSERT INTO %I.%I (id, org_id, table_id, created_at)
       VALUES ($3, $2, $1, now())
       ON CONFLICT (id) DO UPDATE SET %s',
       sname, tname, upd_list)
    USING p_table_id, p_org_id, p_row_id;
  ELSE
    EXECUTE format(
      'INSERT INTO %I.%I (id, org_id, table_id, created_at, %s)
       SELECT $3, $2, $1, now(), %s
       ON CONFLICT (id) DO UPDATE SET %s',
      sname, tname, col_list, expr_list, upd_list)
    USING p_table_id, p_org_id, p_row_id, p_values;
  END IF;
END;
$$;

-- Delete physical row (best-effort)
CREATE OR REPLACE FUNCTION app.delete_row_physical(
  p_table_id bigint,
  p_org_id uuid,
  p_row_id uuid
) RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  sname text;
  tname text;
BEGIN
  SELECT COALESCE(schema_name,'app_data'), COALESCE(physical_table_name, 't_'||p_table_id::text)
  INTO sname, tname
  FROM app.tables
  WHERE id = p_table_id;

  -- If table metadata is missing, nothing to do
  IF sname IS NULL OR tname IS NULL THEN
    RETURN;
  END IF;

  -- Best-effort; if table is not yet created, ensure then delete
  PERFORM app.ensure_physical_table(p_table_id);

  EXECUTE format('DELETE FROM %I.%I WHERE id = $1 AND table_id = $2 AND org_id = $3', sname, tname)
    USING p_row_id, p_table_id, p_org_id;
END;
$$;

COMMIT;

