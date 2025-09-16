CREATE OR REPLACE FUNCTION app.rows_json(p_table_id bigint)
RETURNS TABLE (row_id uuid, data jsonb)
LANGUAGE sql AS $$
  SELECT r.id,
         COALESCE((
           SELECT jsonb_object_agg(v.name, v.val)
           FROM (
             -- build name->typed jsonb value for each column present on the row
             SELECT c.name, to_jsonb(vt.value) val
             FROM app.values_text vt
             JOIN app.columns c ON c.id = vt.column_id
             WHERE vt.row_id = r.id
             UNION ALL
             SELECT c.name, to_jsonb(vf.value)
             FROM app.values_float vf
             JOIN app.columns c ON c.id = vf.column_id
             WHERE vf.row_id = r.id
             UNION ALL
             SELECT c.name, to_jsonb(vd.value)
             FROM app.values_date vd
             JOIN app.columns c ON c.id = vd.column_id
             WHERE vd.row_id = r.id
             UNION ALL
             SELECT c.name, to_jsonb(vb.value)
             FROM app.values_bool vb
             JOIN app.columns c ON c.id = vb.column_id
             WHERE vb.row_id = r.id
             UNION ALL
             SELECT c.name, to_jsonb(ve.value)
             FROM app.values_enum ve
             JOIN app.columns c ON c.id = ve.column_id
             WHERE ve.row_id = r.id
             UNION ALL
             SELECT c.name, to_jsonb(vu.value)
             FROM app.values_uuid vu
             JOIN app.columns c ON c.id = vu.column_id
             WHERE vu.row_id = r.id
           ) v
         ), '{}'::jsonb) AS data
  FROM app.rows r
  WHERE r.table_id = p_table_id
  ORDER BY r.created_at DESC;
$$;
