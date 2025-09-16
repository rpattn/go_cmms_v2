-- Read cutover: provide app.row_to_json that prefers physical storage

BEGIN;

CREATE OR REPLACE FUNCTION app.row_to_json(p_row_id uuid)
RETURNS jsonb
LANGUAGE plpgsql
AS $$
DECLARE
  v_table_id bigint;
  v_org_id uuid;
  sname text;
  tname text;
  j jsonb;
BEGIN
  -- Resolve table/org and physical identifiers
  SELECT r.table_id, t.org_id, COALESCE(t.schema_name,'app_data'), COALESCE(t.physical_table_name, 't_'||r.table_id::text)
  INTO v_table_id, v_org_id, sname, tname
  FROM app.rows r
  JOIN app.tables t ON t.id = r.table_id
  WHERE r.id = p_row_id;

  IF v_table_id IS NULL THEN
    RETURN NULL;
  END IF;

  -- Try to read from physical table first; if table not present yet, fall back
  BEGIN
    EXECUTE format(
      'SELECT COALESCE((
          SELECT jsonb_object_agg(c.name, (to_jsonb(x))->(COALESCE(c.physical_column_name, ''c_''||c.id::text)))
          FROM app.columns c
          WHERE c.table_id = $1
            AND (to_jsonb(x))->(COALESCE(c.physical_column_name, ''c_''||c.id::text)) IS NOT NULL
        ), ''{}''::jsonb)
       FROM %I.%I x
       WHERE x.id = $2 AND x.table_id = $1 AND x.org_id = $3',
      sname, tname)
    INTO j
    USING v_table_id, p_row_id, v_org_id;
  EXCEPTION WHEN undefined_table THEN
    j := NULL;
  END;

  IF j IS NOT NULL THEN
    RETURN j;
  END IF;

  -- Fallback to EAV assembly
  RETURN (
    SELECT data FROM app.rows_json(v_table_id) WHERE row_id = p_row_id
  );
END;
$$;

COMMIT;

