-- Search user table via physical storage

BEGIN;

CREATE OR REPLACE FUNCTION app.search_user_table_physical(
  p_table_name text,
  p_org_id uuid,
  p_payload jsonb
) RETURNS TABLE (row_id uuid, row_data jsonb, total_count bigint)
LANGUAGE plpgsql
AS $$
DECLARE
  v_table_id bigint;
  sname text;
  tname text;
  v_storage app.storage_mode;
  page_num int := GREATEST(0, COALESCE((p_payload->>'pageNum')::int, 0));
  page_size int := GREATEST(1, LEAST(COALESCE((p_payload->>'pageSize')::int, 10), 100));
  base_sql text;
  where_sql text := '';
  final_sql text;
  rec RECORD;
  colrec RECORD;
  op text;
  val text;
  arr text[];
  arr_list text;
  cond text;
  has_filters boolean := false;
BEGIN
  -- Resolve table id and physical identifiers
  SELECT id, COALESCE(schema_name,'app_data'), COALESCE(physical_table_name, 't_'||id::text), storage_mode
  INTO v_table_id, sname, tname, v_storage
  FROM app.tables
  WHERE (slug = lower(p_table_name) OR lower(name) = lower(p_table_name))
    AND org_id = p_org_id
  ORDER BY 1
  LIMIT 1;

  IF v_table_id IS NULL THEN
    RETURN; -- empty
  END IF;

  -- If table not yet cut over to relational, run EAV-based filtering to preserve semantics
  IF v_storage IS NULL OR v_storage <> 'relational' THEN
    RETURN QUERY
    WITH ff AS (
      SELECT jsonb_array_elements(p_payload->'filterFields') AS f
      WHERE (p_payload ? 'filterFields') AND jsonb_typeof(p_payload->'filterFields') = 'array'
    ),
    filtered AS (
      SELECT 
        b.id,
        b.created_at,
        COUNT(*) OVER() AS total_count
      FROM app.rows b
      JOIN app.tables t ON t.id = b.table_id
      WHERE b.table_id = v_table_id AND t.org_id = p_org_id
        AND (
          NOT EXISTS (SELECT 1 FROM ff) OR
          EXISTS (
            SELECT 1
            FROM ff
            LEFT JOIN app.columns c ON c.table_id = v_table_id
              AND lower(c.name) = lower((f->>'field'))
            WHERE 
              CASE
                WHEN c.type = 'text' THEN EXISTS (
                  SELECT 1 FROM app.values_text vt
                  WHERE vt.row_id = b.id AND vt.column_id = c.id AND (
                    CASE COALESCE(f->>'operation','eq')
                      WHEN 'eq' THEN vt.value = (f->>'value')
                      WHEN 'cn' THEN vt.value ILIKE '%' || (f->>'value') || '%'
                      WHEN 'in' THEN vt.value = ANY(ARRAY(SELECT jsonb_array_elements_text(f->'values')))
                      ELSE TRUE
                    END
                  )
                )
                WHEN c.type = 'enum' THEN EXISTS (
                  SELECT 1 FROM app.values_enum ve
                  WHERE ve.row_id = b.id AND ve.column_id = c.id AND (
                    CASE COALESCE(f->>'operation','eq')
                      WHEN 'eq' THEN ve.value = (f->>'value')
                      WHEN 'in' THEN ve.value = ANY(ARRAY(SELECT jsonb_array_elements_text(f->'values')))
                      ELSE TRUE
                    END
                  )
                )
                WHEN c.type = 'bool' THEN EXISTS (
                  SELECT 1 FROM app.values_bool vb
                  WHERE vb.row_id = b.id AND vb.column_id = c.id 
                  AND vb.value IS NOT DISTINCT FROM ((f->>'value')::boolean)
                )
                ELSE TRUE
              END
          )
        )
    )
    SELECT f.id AS row_id,
           app.row_to_json(f.id) AS row_data,
           f.total_count
    FROM filtered f
    ORDER BY f.created_at DESC
    LIMIT page_size OFFSET page_size * page_num;
    RETURN;
  END IF;

  PERFORM app.ensure_physical_table(v_table_id);

  -- Build base SQL using spaces (avoid backslash escapes like \n which break EXECUTE)
  base_sql := format('SELECT x.id AS row_id, app.row_to_json(x.id) AS row_data, COUNT(*) OVER() AS total_count '
                    ||'FROM %I.%I x '
                    ||'WHERE x.table_id = %s AND x.org_id = %L',
                     sname, tname, v_table_id, p_org_id);

  -- Build WHERE from payload.filterFields
  FOR rec IN
    SELECT jsonb_array_elements(p_payload->'filterFields') AS f
    WHERE (p_payload ? 'filterFields') AND jsonb_typeof(p_payload->'filterFields') = 'array'
  LOOP
    has_filters := true;
    SELECT c.id, c.name, c.type::text AS type,
           COALESCE(c.physical_column_name, 'c_'||c.id::text) AS phys_name,
           COALESCE(c.enum_type_name, format('%I.%I', COALESCE(t.schema_name,'app_data'), 'e_'||c.id::text)) AS enum_type
    INTO colrec
    FROM app.columns c
    JOIN app.tables t ON t.id = c.table_id
    WHERE c.table_id = v_table_id AND lower(c.name) = lower(rec.f->>'field')
    LIMIT 1;

    -- If no row matched, the RECORD has no type; use FOUND to skip safely
    IF NOT FOUND THEN
      CONTINUE; -- unknown field -> ignore filter
    END IF;

    -- Ensure physical column exists (covers pre-migration columns)
    PERFORM app.add_physical_column(colrec.id);

    op := COALESCE(rec.f->>'operation','eq');
    cond := NULL;

    IF colrec.type = 'text' THEN
      IF op = 'eq' THEN
        val := rec.f->>'value';
        IF val IS NOT NULL THEN
          cond := format('x.%I = %L', colrec.phys_name, val);
        END IF;
      ELSIF op = 'cn' THEN
        val := rec.f->>'value';
        IF val IS NOT NULL THEN
          -- Simpler and safer: build the pattern as a single literal
          cond := format('x.%I ILIKE %L', colrec.phys_name, '%' || val || '%');
        END IF;
      ELSIF op = 'in' THEN
        arr := ARRAY(SELECT jsonb_array_elements_text(rec.f->'values'));
        IF arr IS NULL OR array_length(arr,1) IS NULL THEN
          cond := 'FALSE';
        ELSE
          SELECT string_agg(quote_literal(v), ',') INTO arr_list FROM unnest(arr) v;
          cond := format('x.%I = ANY(ARRAY[%s])', colrec.phys_name, arr_list);
        END IF;
      END IF;
    ELSIF colrec.type = 'enum' THEN
      IF op = 'eq' THEN
        val := rec.f->>'value';
        IF val IS NOT NULL THEN
          cond := format('x.%I = %L::%s', colrec.phys_name, val, colrec.enum_type);
        END IF;
      ELSIF op = 'in' THEN
        arr := ARRAY(SELECT jsonb_array_elements_text(rec.f->'values'));
        IF arr IS NULL OR array_length(arr,1) IS NULL THEN
          cond := 'FALSE';
        ELSE
          SELECT string_agg(quote_literal(v), ',') INTO arr_list FROM unnest(arr) v;
          cond := format('x.%I = ANY(ARRAY[%s]::%s[])', colrec.phys_name, arr_list, colrec.enum_type);
        END IF;
      END IF;
    ELSIF colrec.type = 'bool' THEN
      val := rec.f->>'value';
      IF val IS NOT NULL THEN
        cond := format('x.%I IS NOT DISTINCT FROM %L::boolean', colrec.phys_name, val);
      END IF;
    END IF;

    IF cond IS NOT NULL THEN
      where_sql := where_sql || ' AND ' || cond;
    END IF;
  END LOOP;

  final_sql := base_sql || where_sql || format(' ORDER BY x.created_at DESC LIMIT %s OFFSET %s', page_size, page_num * page_size);

  RETURN QUERY EXECUTE final_sql;
END;
$$;

COMMIT;
