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
  sort_dir text;
  sort_field text;
  order_sql text;
  ordrec RECORD;
  eav_sql text;
BEGIN
  -- Resolve table id and physical identifiers
  SELECT id, COALESCE(schema_name,'app_data'), COALESCE(physical_table_name, 't_'||id::text), storage_mode
  INTO v_table_id, sname, tname, v_storage
  FROM app.tables t
  WHERE (t.slug = lower(p_table_name) OR lower(t.name) = lower(p_table_name))
    AND (t.org_id = p_org_id OR t.org_id IS NULL)
  ORDER BY CASE WHEN t.org_id = p_org_id THEN 0 ELSE 1 END
  LIMIT 1;

  IF v_table_id IS NULL THEN
    RETURN; -- empty
  END IF;

  -- Parse sort parameters. Default to created_at DESC.
  sort_dir := upper(COALESCE(NULLIF(p_payload->>'direction',''),'DESC'));
  IF sort_dir NOT IN ('ASC','DESC') THEN sort_dir := 'DESC'; END IF;
  sort_field := lower(NULLIF(p_payload->>'sortField',''));

  -- If table not yet cut over to relational, run EAV-based filtering to preserve semantics
  IF v_storage IS NULL OR v_storage <> 'relational' THEN
    -- Decide sort strategy in EAV mode
    IF sort_field IS NULL OR sort_field = '' OR sort_field = 'created_at' OR sort_field = 'updated_at' THEN
      -- Sort by timestamps only (stable and cheap)
      IF sort_dir = 'ASC' THEN
        RETURN QUERY
        WITH ff AS (
          SELECT jsonb_array_elements(p_payload->'filterFields') AS f
          WHERE (p_payload ? 'filterFields') AND jsonb_typeof(p_payload->'filterFields') = 'array'
        ),
        filtered AS (
          SELECT 
            b.id,
            b.created_at,
            b.updated_at,
            COUNT(*) OVER() AS total_count
          FROM app.rows b
          JOIN app.tables t ON t.id = b.table_id
          WHERE b.table_id = v_table_id AND (t.org_id = p_org_id OR t.org_id IS NULL)
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
        ORDER BY CASE WHEN sort_field = 'updated_at' THEN f.updated_at ELSE f.created_at END ASC
        LIMIT page_size OFFSET page_size * page_num;
        RETURN;
      ELSE
        RETURN QUERY
        WITH ff AS (
          SELECT jsonb_array_elements(p_payload->'filterFields') AS f
          WHERE (p_payload ? 'filterFields') AND jsonb_typeof(p_payload->'filterFields') = 'array'
        ),
        filtered AS (
          SELECT 
            b.id,
            b.created_at,
            b.updated_at,
            COUNT(*) OVER() AS total_count
          FROM app.rows b
          JOIN app.tables t ON t.id = b.table_id
          WHERE b.table_id = v_table_id AND (t.org_id = p_org_id OR t.org_id IS NULL)
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
      ORDER BY CASE WHEN sort_field = 'updated_at' THEN f.updated_at ELSE f.created_at END DESC
         LIMIT page_size OFFSET page_size * page_num;
         RETURN;
      END IF;
    ELSE
      -- Sort by an EAV value column; compute per-type value and order by it
      SELECT c.id, c.type::text AS type INTO ordrec
      FROM app.columns c
      WHERE c.table_id = v_table_id AND lower(c.name) = sort_field
      LIMIT 1;
      IF NOT FOUND THEN
        -- fallback to timestamps
        sort_field := NULL;
        IF sort_dir = 'ASC' THEN
          RETURN QUERY SELECT * FROM (
            WITH ff AS (
              SELECT jsonb_array_elements(p_payload->'filterFields') AS f
              WHERE (p_payload ? 'filterFields') AND jsonb_typeof(p_payload->'filterFields') = 'array'
            ),
            filtered AS (
              SELECT b.id, b.created_at, b.updated_at, COUNT(*) OVER() AS total_count
              FROM app.rows b JOIN app.tables t ON t.id = b.table_id
              WHERE b.table_id = v_table_id AND (t.org_id = p_org_id OR t.org_id IS NULL) AND (
                NOT EXISTS (SELECT 1 FROM ff) OR EXISTS (
                  SELECT 1 FROM ff LEFT JOIN app.columns c ON c.table_id = v_table_id AND lower(c.name) = lower((f->>'field'))
                  WHERE CASE WHEN c.type='text' THEN EXISTS (
                    SELECT 1 FROM app.values_text vt WHERE vt.row_id=b.id AND vt.column_id=c.id AND (
                      CASE COALESCE(f->>'operation','eq') WHEN 'eq' THEN vt.value=(f->>'value') WHEN 'cn' THEN vt.value ILIKE '%'||(f->>'value')||'%' WHEN 'in' THEN vt.value = ANY(ARRAY(SELECT jsonb_array_elements_text(f->'values'))) ELSE TRUE END)
                  ) WHEN c.type='enum' THEN EXISTS (
                    SELECT 1 FROM app.values_enum ve WHERE ve.row_id=b.id AND ve.column_id=c.id AND (
                      CASE COALESCE(f->>'operation','eq') WHEN 'eq' THEN ve.value=(f->>'value') WHEN 'in' THEN ve.value = ANY(ARRAY(SELECT jsonb_array_elements_text(f->'values'))) ELSE TRUE END)
                  ) WHEN c.type='bool' THEN EXISTS (
                    SELECT 1 FROM app.values_bool vb WHERE vb.row_id=b.id AND vb.column_id=c.id AND vb.value IS NOT DISTINCT FROM ((f->>'value')::boolean)
                  ) ELSE TRUE END)
              )
            ) SELECT f.id AS row_id, app.row_to_json(f.id) AS row_data, f.total_count FROM filtered f ORDER BY f.created_at ASC LIMIT page_size OFFSET page_size*page_num
          ) s; RETURN;
        ELSE
          RETURN QUERY SELECT * FROM (
            WITH ff AS (
              SELECT jsonb_array_elements(p_payload->'filterFields') AS f
              WHERE (p_payload ? 'filterFields') AND jsonb_typeof(p_payload->'filterFields') = 'array'
            ),
            filtered AS (
              SELECT b.id, b.created_at, b.updated_at, COUNT(*) OVER() AS total_count
              FROM app.rows b JOIN app.tables t ON t.id = b.table_id
              WHERE b.table_id = v_table_id AND (t.org_id = p_org_id OR t.org_id IS NULL) AND (
                NOT EXISTS (SELECT 1 FROM ff) OR EXISTS (
                  SELECT 1 FROM ff LEFT JOIN app.columns c ON c.table_id = v_table_id AND lower(c.name) = lower((f->>'field'))
                  WHERE CASE WHEN c.type='text' THEN EXISTS (
                    SELECT 1 FROM app.values_text vt WHERE vt.row_id=b.id AND vt.column_id=c.id AND (
                      CASE COALESCE(f->>'operation','eq') WHEN 'eq' THEN vt.value=(f->>'value') WHEN 'cn' THEN vt.value ILIKE '%'||(f->>'value')||'%' WHEN 'in' THEN vt.value = ANY(ARRAY(SELECT jsonb_array_elements_text(f->'values'))) ELSE TRUE END)
                  ) WHEN c.type='enum' THEN EXISTS (
                    SELECT 1 FROM app.values_enum ve WHERE ve.row_id=b.id AND ve.column_id=c.id AND (
                      CASE COALESCE(f->>'operation','eq') WHEN 'eq' THEN ve.value=(f->>'value') WHEN 'in' THEN ve.value = ANY(ARRAY(SELECT jsonb_array_elements_text(f->'values'))) ELSE TRUE END)
                  ) WHEN c.type='bool' THEN EXISTS (
                    SELECT 1 FROM app.values_bool vb WHERE vb.row_id=b.id AND vb.column_id=c.id AND vb.value IS NOT DISTINCT FROM ((f->>'value')::boolean)
                  ) ELSE TRUE END)
              )
            ) SELECT f.id AS row_id, app.row_to_json(f.id) AS row_data, f.total_count FROM filtered f ORDER BY f.created_at DESC LIMIT page_size OFFSET page_size*page_num
          ) s; RETURN;
        END IF;
      END IF;

      -- Build a typed sort value column and order by it
      eav_sql := 'WITH ff AS (
        SELECT jsonb_array_elements($1->''filterFields'') AS f
        WHERE ($1 ? ''filterFields'') AND jsonb_typeof($1->''filterFields'') = ''array''
      ),
      filtered AS (
        SELECT b.id, b.created_at, b.updated_at,
               ';
      IF ordrec.type = 'text' THEN
        eav_sql := eav_sql || format('(SELECT vt.value FROM app.values_text vt WHERE vt.row_id = b.id AND vt.column_id = %s) AS sort_v,', ordrec.id);
      ELSIF ordrec.type = 'enum' THEN
        eav_sql := eav_sql || format('(SELECT ve.value FROM app.values_enum ve WHERE ve.row_id = b.id AND ve.column_id = %s) AS sort_v,', ordrec.id);
      ELSIF ordrec.type = 'bool' THEN
        eav_sql := eav_sql || format('(SELECT vb.value FROM app.values_bool vb WHERE vb.row_id = b.id AND vb.column_id = %s) AS sort_v,', ordrec.id);
      ELSIF ordrec.type = 'date' THEN
        eav_sql := eav_sql || format('(SELECT vd.value FROM app.values_date vd WHERE vd.row_id = b.id AND vd.column_id = %s) AS sort_v,', ordrec.id);
      ELSIF ordrec.type = 'float' THEN
        eav_sql := eav_sql || format('(SELECT vf.value FROM app.values_float vf WHERE vf.row_id = b.id AND vf.column_id = %s) AS sort_v,', ordrec.id);
      ELSIF ordrec.type = 'uuid' THEN
        eav_sql := eav_sql || format('(SELECT vu.value FROM app.values_uuid vu WHERE vu.row_id = b.id AND vu.column_id = %s) AS sort_v,', ordrec.id);
      ELSE
        eav_sql := eav_sql || 'NULL::text AS sort_v,';
      END IF;
      eav_sql := eav_sql || ' COUNT(*) OVER() AS total_count
        FROM app.rows b JOIN app.tables t ON t.id = b.table_id
        WHERE b.table_id = $2 AND (t.org_id = $3 OR t.org_id IS NULL)
          AND (
            NOT EXISTS (SELECT 1 FROM ff) OR
            EXISTS (
              SELECT 1 FROM ff
              LEFT JOIN app.columns c ON c.table_id = $2 AND lower(c.name) = lower((f->>''field''))
              WHERE CASE
                WHEN c.type = ''text'' THEN EXISTS (
                  SELECT 1 FROM app.values_text vt
                  WHERE vt.row_id = b.id AND vt.column_id = c.id AND (
                    CASE COALESCE(f->>''operation'',''eq'')
                      WHEN ''eq'' THEN vt.value = (f->>''value'')
                      WHEN ''cn'' THEN vt.value ILIKE ''%'' || (f->>''value'') || ''%''
                      WHEN ''in'' THEN vt.value = ANY(ARRAY(SELECT jsonb_array_elements_text(f->''values'')))
                      ELSE TRUE
                    END))
                WHEN c.type = ''enum'' THEN EXISTS (
                  SELECT 1 FROM app.values_enum ve
                  WHERE ve.row_id = b.id AND ve.column_id = c.id AND (
                    CASE COALESCE(f->>''operation'',''eq'')
                      WHEN ''eq'' THEN ve.value = (f->>''value'')
                      WHEN ''in'' THEN ve.value = ANY(ARRAY(SELECT jsonb_array_elements_text(f->''values'')))
                      ELSE TRUE
                    END))
                WHEN c.type = ''bool'' THEN EXISTS (
                  SELECT 1 FROM app.values_bool vb
                  WHERE vb.row_id = b.id AND vb.column_id = c.id AND vb.value IS NOT DISTINCT FROM ((f->>''value'')::boolean))
                ELSE TRUE END))
      )
      SELECT f.id AS row_id, app.row_to_json(f.id) AS row_data, f.total_count
      FROM filtered f ';
      IF sort_dir = 'ASC' THEN
        eav_sql := eav_sql || ' ORDER BY f.sort_v ASC NULLS LAST, f.created_at DESC';
      ELSE
        eav_sql := eav_sql || ' ORDER BY f.sort_v DESC NULLS LAST, f.created_at DESC';
      END IF;
      eav_sql := eav_sql || ' LIMIT $4 OFFSET $5';

      RETURN QUERY EXECUTE eav_sql USING p_payload, v_table_id, p_org_id, page_size, page_num * page_size;
      RETURN;
    END IF;
  END IF;

  PERFORM app.ensure_physical_table(v_table_id);

  -- Build base SQL using spaces (avoid backslash escapes like \n which break EXECUTE)
  base_sql := format('SELECT x.id AS row_id, app.row_to_json(x.id) AS row_data, COUNT(*) OVER() AS total_count '
                    ||'FROM %I.%I x '
                    ||'WHERE x.table_id = %s AND (x.org_id = %L OR x.org_id IS NULL)',
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

  -- For relational storage, support sorting by created_at/updated_at or a logical column
  IF sort_field = 'updated_at' THEN
    order_sql := format(' ORDER BY x.updated_at %s', sort_dir);
  ELSIF sort_field = 'created_at' OR sort_field IS NULL THEN
    order_sql := format(' ORDER BY x.created_at %s', sort_dir);
  ELSE
    -- Lookup the requested sort column and map to its physical column
    SELECT c.id, c.type::text AS type,
           COALESCE(c.physical_column_name, 'c_'||c.id::text) AS phys_name,
           COALESCE(c.enum_type_name, format('%I.%I', COALESCE(t.schema_name,'app_data'), 'e_'||c.id::text)) AS enum_type
    INTO ordrec
    FROM app.columns c
    JOIN app.tables t ON t.id = c.table_id
    WHERE c.table_id = v_table_id AND lower(c.name) = sort_field
    LIMIT 1;
    IF FOUND THEN
      -- ensure the physical column exists
      PERFORM app.add_physical_column(ordrec.id);
      order_sql := format(' ORDER BY x.%I %s', ordrec.phys_name, sort_dir);
    ELSE
      order_sql := format(' ORDER BY x.created_at %s', sort_dir);
    END IF;
  END IF;

  final_sql := base_sql || where_sql || order_sql || format(' LIMIT %s OFFSET %s', page_size, page_num * page_size);

  RETURN QUERY EXECUTE final_sql;
END;
$$;

COMMIT;
