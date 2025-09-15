
-- Add this at the end of the file:

-- Function to build a JSON object from row values
CREATE OR REPLACE FUNCTION app.row_to_json(p_row_id uuid)
RETURNS jsonb
LANGUAGE plpgsql
AS $$
DECLARE
    result jsonb := '{}'::jsonb;
BEGIN
    -- Add text values
    SELECT COALESCE(result, '{}'::jsonb) || COALESCE(jsonb_object_agg(c.name, v.value), '{}'::jsonb)
    INTO result
    FROM app.values_text v
    JOIN app.columns c ON c.id = v.column_id
    WHERE v.row_id = p_row_id;

    -- Add float values
    SELECT COALESCE(result, '{}'::jsonb) || COALESCE(jsonb_object_agg(c.name, v.value), '{}'::jsonb)
    INTO result
    FROM app.values_float v
    JOIN app.columns c ON c.id = v.column_id
    WHERE v.row_id = p_row_id;

    -- Add date values
    SELECT COALESCE(result, '{}'::jsonb) || COALESCE(jsonb_object_agg(c.name, v.value), '{}'::jsonb)
    INTO result
    FROM app.values_date v
    JOIN app.columns c ON c.id = v.column_id
    WHERE v.row_id = p_row_id;

    -- Add boolean values
    SELECT COALESCE(result, '{}'::jsonb) || COALESCE(jsonb_object_agg(c.name, v.value), '{}'::jsonb)
    INTO result
    FROM app.values_bool v
    JOIN app.columns c ON c.id = v.column_id
    WHERE v.row_id = p_row_id;

    -- Add enum values
    SELECT COALESCE(result, '{}'::jsonb) || COALESCE(jsonb_object_agg(c.name, v.value), '{}'::jsonb)
    INTO result
    FROM app.values_enum v
    JOIN app.columns c ON c.id = v.column_id
    WHERE v.row_id = p_row_id;

    -- Add UUID reference values
    SELECT COALESCE(result, '{}'::jsonb) || COALESCE(jsonb_object_agg(c.name, v.value), '{}'::jsonb)
    INTO result
    FROM app.values_uuid v
    JOIN app.columns c ON c.id = v.column_id
    WHERE v.row_id = p_row_id;

    -- Add metadata
    SELECT COALESCE(result, '{}'::jsonb) || COALESCE(jsonb_build_object(
        'id', r.id,
        'created_at', r.created_at
    ), '{}'::jsonb)
    INTO result
    FROM app.rows r
    WHERE r.id = p_row_id;

    RETURN COALESCE(result, '{}'::jsonb);
END;
$$;
