-- Ensure insert_row_physical creates missing physical columns on the fly

BEGIN;

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
  lock_id bigint;
BEGIN
  -- Acquire table metadata locks in deterministic order to avoid deadlocks
  FOR lock_id IN (
    SELECT DISTINCT id
    FROM (
      SELECT p_table_id AS id
      UNION ALL
      SELECT c.reference_table_id
      FROM app.columns c
      WHERE c.table_id = p_table_id
        AND c.reference_table_id IS NOT NULL
    ) AS ids
    WHERE id IS NOT NULL
    ORDER BY id
  ) LOOP
    PERFORM 1 FROM app.tables WHERE id = lock_id FOR UPDATE;
  END LOOP;

  PERFORM app.ensure_physical_table(p_table_id);

  SELECT COALESCE(t.schema_name,'app_data'), COALESCE(t.physical_table_name, 't_'||p_table_id::text)
  INTO sname, tname
  FROM app.tables t
  WHERE t.id = p_table_id;

  FOR rec IN (
    SELECT c.id AS col_id, c.name AS logical_name, COALESCE(c.physical_column_name, 'c_'||c.id::text) AS phys_name,
           c.type, COALESCE(c.enum_type_name, format('%I.%I', sname, 'e_'||c.id::text)) AS enum_type
    FROM app.columns c
    WHERE c.table_id = p_table_id
    ORDER BY c.id
  ) LOOP
    -- Ensure the physical column exists for each logical column
    PERFORM app.add_physical_column(rec.col_id);

    IF NOT first THEN
      col_list := col_list || ', ';
      expr_list := expr_list || ', ';
    END IF;
    -- Always separate updates from the static prefixes (org_id, table_id)
    upd_list := upd_list || ', ';
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

COMMIT;
