-- Add updated_at to logical rows and ensure timestamps are present in physical tables

BEGIN;

-- 1) Ensure updated_at exists on EAV rows
ALTER TABLE app.rows
  ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now();

-- 2) Generic trigger to touch updated_at
CREATE OR REPLACE FUNCTION app.touch_updated_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  NEW.updated_at := now();
  RETURN NEW;
END;
$$;

-- 3) Ensure physical tables include updated_at and have trigger
CREATE OR REPLACE FUNCTION app.ensure_physical_table(p_table_id bigint)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  sname text;
  tname text;
  -- serialize DDL for this table id to avoid overlapping CREATE TABLEs grabbing
  -- conflicting ShareRowExclusive locks. This mirrors the behaviour in the
  -- original rollout (021_physical_storage) where we took the same advisory
  -- lock. Without it, concurrent inserts that race to lazily provision the
  -- physical table can deadlock when CREATE TABLE IF NOT EXISTS runs in two
  -- transactions simultaneously.
  PERFORM pg_advisory_xact_lock(
           hashtextextended('app.ensure_physical_table:' || p_table_id::text, 0));

  SELECT COALESCE(schema_name,'app_data'), COALESCE(physical_table_name, 't_'||p_table_id::text)
  INTO sname, tname
  FROM app.tables
  WHERE id = p_table_id
  FOR UPDATE;

  IF sname IS NULL OR length(trim(sname)) = 0 THEN sname := 'app_data'; END IF;
  IF tname IS NULL OR length(trim(tname)) = 0 THEN tname := 't_' || p_table_id::text; END IF;

  UPDATE app.tables SET schema_name = sname, physical_table_name = tname WHERE id = p_table_id;

  EXECUTE format('CREATE SCHEMA IF NOT EXISTS %I', sname);

  -- include updated_at column
  EXECUTE format(
    'CREATE TABLE IF NOT EXISTS %I.%I (
        id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
        org_id uuid NOT NULL REFERENCES organisations(id) ON DELETE RESTRICT,
        table_id bigint NOT NULL REFERENCES app.tables(id) ON DELETE CASCADE,
        created_at timestamptz NOT NULL DEFAULT now(),
        updated_at timestamptz NOT NULL DEFAULT now()
     )', sname, tname);

  EXECUTE format('CREATE INDEX IF NOT EXISTS %I ON %I.%I (org_id)',
                 'ix_'||tname||'_org_id', sname, tname);
  EXECUTE format('CREATE INDEX IF NOT EXISTS %I ON %I.%I (table_id, created_at DESC)',
                 'ix_'||tname||'_table_created', sname, tname);

  -- ensure trigger to touch updated_at
  EXECUTE format('DROP TRIGGER IF EXISTS set_updated_at ON %I.%I', sname, tname);
  EXECUTE format('CREATE TRIGGER set_updated_at BEFORE UPDATE ON %I.%I FOR EACH ROW EXECUTE FUNCTION app.touch_updated_at()', sname, tname);
END;
$$;

-- 4) Update EAV update function to touch updated_at and keep physical in sync
CREATE OR REPLACE FUNCTION app.update_row(p_row_id uuid, p_values jsonb)
RETURNS void LANGUAGE plpgsql AS $$
DECLARE
  t_id bigint;
  org uuid;
  rec record;
  col app.columns;
  eav_data jsonb;
  val_text text; -- scalar unwrapped from JSON (NULL if JSON null)
BEGIN
  SELECT table_id INTO t_id FROM app.rows WHERE id = p_row_id;
  IF t_id IS NULL THEN RAISE EXCEPTION 'Unknown row_id %', p_row_id; END IF;

  FOR rec IN SELECT key AS col_name, value FROM jsonb_each(p_values)
  LOOP
    SELECT * INTO col FROM app.columns WHERE table_id = t_id AND name = rec.col_name;
    IF col.id IS NULL THEN
      RAISE EXCEPTION 'Unknown column "%" for table_id %', rec.col_name, t_id;
    END IF;

    -- Unwrap JSON scalar once (JSON null -> NULL)
    val_text := rec.value #>> '{}';

    IF col.type='text' THEN
      INSERT INTO app.values_text(row_id, column_id, value)
      VALUES (p_row_id, col.id, val_text)
      ON CONFLICT (row_id, column_id) DO UPDATE SET value = EXCLUDED.value;
    ELSIF col.type='date' THEN
      INSERT INTO app.values_date(row_id, column_id, value)
      VALUES (p_row_id, col.id, (val_text)::date)
      ON CONFLICT (row_id, column_id) DO UPDATE SET value = EXCLUDED.value;
    ELSIF col.type='bool' THEN
      INSERT INTO app.values_bool(row_id, column_id, value)
      VALUES (p_row_id, col.id, (val_text)::boolean)
      ON CONFLICT (row_id, column_id) DO UPDATE SET value = EXCLUDED.value;
    ELSIF col.type='enum' THEN
      INSERT INTO app.values_enum(row_id, column_id, value)
      VALUES (p_row_id, col.id, val_text)
      ON CONFLICT (row_id, column_id) DO UPDATE SET value = EXCLUDED.value;
    ELSIF col.type='uuid' THEN
      INSERT INTO app.values_uuid(row_id, column_id, value)
      VALUES (p_row_id, col.id, (val_text)::uuid)
      ON CONFLICT (row_id, column_id) DO UPDATE SET value = EXCLUDED.value;
    ELSIF col.type='float' THEN
      INSERT INTO app.values_float(row_id, column_id, value)
      VALUES (p_row_id, col.id, (val_text)::double precision)
      ON CONFLICT (row_id, column_id) DO UPDATE SET value = EXCLUDED.value;
    END IF;

    IF col.is_indexed THEN
      PERFORM app.ensure_index(col.id);
    END IF;
  END LOOP;

  -- Touch updated_at on logical row
  UPDATE app.rows SET updated_at = now() WHERE id = p_row_id;

  -- Dual-write to physical
  SELECT t.org_id INTO org FROM app.rows r JOIN app.tables t ON t.id = r.table_id WHERE r.id = p_row_id;
  IF org IS NOT NULL THEN
    PERFORM app.ensure_physical_table(t_id);
    -- Build canonical JSON from EAV only (avoid physical-first path to prevent stale writes)
    SELECT data INTO eav_data FROM app.rows_json(t_id) WHERE row_id = p_row_id;
    PERFORM app.insert_row_physical(t_id, org, p_row_id, COALESCE(eav_data, '{}'::jsonb));
  END IF;
END$$;

-- 5) Include timestamps in row JSON
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
  SELECT r.table_id, t.org_id, COALESCE(t.schema_name,'app_data'), COALESCE(t.physical_table_name, 't_'||r.table_id::text)
  INTO v_table_id, v_org_id, sname, tname
  FROM app.rows r
  JOIN app.tables t ON t.id = r.table_id
  WHERE r.id = p_row_id;

  IF v_table_id IS NULL THEN
    RETURN NULL;
  END IF;

  BEGIN
    EXECUTE format(
      'SELECT (
          COALESCE((
            SELECT jsonb_object_agg(c.name, (to_jsonb(x))->(COALESCE(c.physical_column_name, ''c_''||c.id::text)))
            FROM app.columns c
            WHERE c.table_id = $1
              AND (to_jsonb(x))->(COALESCE(c.physical_column_name, ''c_''||c.id::text)) IS NOT NULL
          ), ''{}''::jsonb)
          || jsonb_build_object(''created_at'', x.created_at, ''updated_at'', x.updated_at)
        )
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

  RETURN (
    SELECT COALESCE(d.data, '{}'::jsonb) || jsonb_build_object('created_at', r.created_at, 'updated_at', r.updated_at)
    FROM app.rows r
    LEFT JOIN LATERAL (
      SELECT data FROM app.rows_json(v_table_id) WHERE row_id = p_row_id
    ) d ON TRUE
    WHERE r.id = p_row_id
  );
END;
$$;

COMMIT;
