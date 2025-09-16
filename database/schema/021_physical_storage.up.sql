-- Table-per-Collection: initial physical storage scaffolding
-- - Adds metadata to app.tables/app.columns
-- - Creates helper schema and DDL helpers for physical tables/columns

BEGIN;

-- Create dedicated schema for user data tables
CREATE SCHEMA IF NOT EXISTS app_data;

-- Storage mode enum
DO $$ BEGIN
  CREATE TYPE app.storage_mode AS ENUM ('eav','relational');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- app.tables metadata for physical storage
ALTER TABLE app.tables
  ADD COLUMN IF NOT EXISTS schema_name text NOT NULL DEFAULT 'app_data',
  ADD COLUMN IF NOT EXISTS physical_table_name text,
  ADD COLUMN IF NOT EXISTS storage_mode app.storage_mode NOT NULL DEFAULT 'eav',
  ADD COLUMN IF NOT EXISTS migrated_at timestamptz;

-- app.columns metadata for physical storage
ALTER TABLE app.columns
  ADD COLUMN IF NOT EXISTS physical_column_name text,
  ADD COLUMN IF NOT EXISTS enum_type_name text;

-- Helper to get schema-qualified physical table ident and ensure metadata defaults
CREATE OR REPLACE FUNCTION app.physical_table_ident(p_table_id bigint)
RETURNS text
LANGUAGE plpgsql
AS $$
DECLARE
  sname text;
  tname text;
BEGIN
  SELECT COALESCE(schema_name, 'app_data'), physical_table_name
  INTO sname, tname
  FROM app.tables
  WHERE id = p_table_id
  FOR UPDATE;

  IF tname IS NULL OR length(trim(tname)) = 0 THEN
    tname := 't_' || p_table_id::text;
    UPDATE app.tables SET physical_table_name = tname WHERE id = p_table_id;
  END IF;
  IF sname IS NULL OR length(trim(sname)) = 0 THEN
    sname := 'app_data';
    UPDATE app.tables SET schema_name = sname WHERE id = p_table_id;
  END IF;
  RETURN format('%I.%I', sname, tname);
END;
$$;

-- Ensure a physical table exists with base columns
CREATE OR REPLACE FUNCTION app.ensure_physical_table(p_table_id bigint)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  qname text;  -- schema-qualified identifier
  sname text;
  tname text;
BEGIN
  -- lock and resolve identifiers
  SELECT COALESCE(schema_name,'app_data'), COALESCE(physical_table_name, 't_'||p_table_id::text)
  INTO sname, tname
  FROM app.tables
  WHERE id = p_table_id
  FOR UPDATE;

  IF sname IS NULL OR length(trim(sname)) = 0 THEN sname := 'app_data'; END IF;
  IF tname IS NULL OR length(trim(tname)) = 0 THEN tname := 't_' || p_table_id::text; END IF;

  -- persist any computed defaults
  UPDATE app.tables SET schema_name = sname, physical_table_name = tname WHERE id = p_table_id;

  -- create schema if needed
  EXECUTE format('CREATE SCHEMA IF NOT EXISTS %I', sname);

  -- create table if needed
  EXECUTE format(
    'CREATE TABLE IF NOT EXISTS %I.%I (
        id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
        org_id uuid NOT NULL REFERENCES organisations(id) ON DELETE RESTRICT,
        table_id bigint NOT NULL REFERENCES app.tables(id) ON DELETE CASCADE,
        created_at timestamptz NOT NULL DEFAULT now()
     )', sname, tname);

  -- base index for org scoping
  EXECUTE format('CREATE INDEX IF NOT EXISTS %I ON %I.%I (org_id)',
                 'ix_'||tname||'_org_id', sname, tname);

  -- optional: composite for common access patterns
  EXECUTE format('CREATE INDEX IF NOT EXISTS %I ON %I.%I (table_id, created_at DESC)',
                 'ix_'||tname||'_table_created', sname, tname);

  -- mark mode as relational in metadata if previously eav (controlled rollout later)
  -- Do not force flip; callers should manage dual-write/cutover.
END;
$$;

-- Map logical column to physical column and create constraints/indexes
CREATE OR REPLACE FUNCTION app.add_physical_column(p_column_id bigint)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  t_id bigint;
  sname text;
  tname text;
  cname text;
  ctype app.column_type;
  required boolean;
  is_indexed boolean;
  is_ref boolean;
  ref_table_id bigint;
  req_diff boolean;
  enum_vals text[];
  enum_type text;
  sql_type text;
  ref_sname text;
  ref_tname text;
  fk_name text;
BEGIN
  -- fetch column + owning table metadata and lock the column
  SELECT c.table_id, COALESCE(t.schema_name,'app_data'), COALESCE(t.physical_table_name, 't_'||c.table_id::text),
         COALESCE(c.physical_column_name, 'c_'||c.id::text), c.type, c.is_required, c.is_indexed,
         c.is_reference, c.reference_table_id, c.require_different_table,
         c.enum_values, c.enum_type_name
  INTO t_id, sname, tname, cname, ctype, required, is_indexed, is_ref, ref_table_id, req_diff,
       enum_vals, enum_type
  FROM app.columns c
  JOIN app.tables t ON t.id = c.table_id
  WHERE c.id = p_column_id
  FOR UPDATE;

  -- persist computed names
  UPDATE app.columns SET physical_column_name = cname WHERE id = p_column_id;
  UPDATE app.tables  SET schema_name = sname, physical_table_name = tname WHERE id = t_id;

  -- ensure table exists
  PERFORM app.ensure_physical_table(t_id);

  -- resolve SQL type
  IF ctype = 'text' THEN
    sql_type := 'text';
  ELSIF ctype = 'date' THEN
    sql_type := 'date';
  ELSIF ctype = 'bool' THEN
    sql_type := 'boolean';
  ELSIF ctype = 'float' THEN
    sql_type := 'double precision';
  ELSIF ctype = 'uuid' THEN
    sql_type := 'uuid';
  ELSIF ctype = 'enum' THEN
    -- create a dedicated enum type per column if missing
    IF enum_type IS NULL OR length(trim(enum_type)) = 0 THEN
      enum_type := format('%I.%I', sname, 'e_'||p_column_id::text);
      UPDATE app.columns SET enum_type_name = enum_type WHERE id = p_column_id;
    END IF;
    -- if type doesn't exist, create it with current values
    PERFORM 1 FROM pg_type t
      JOIN pg_namespace n ON n.oid = t.typnamespace
     WHERE n.nspname = split_part(enum_type, '.', 1)
       AND t.typname = split_part(enum_type, '.', 2);
    IF NOT FOUND THEN
      EXECUTE format('CREATE TYPE %s AS ENUM (%s)'
        , enum_type
        , COALESCE(
            (SELECT string_agg(quote_literal(v), ',') FROM unnest(enum_vals) v),
            '''__placeholder__'''  -- fallback to allow creation even if empty; to be amended later
          ));
    END IF;
    sql_type := enum_type; -- use fully qualified type
  ELSE
    RAISE EXCEPTION 'Unsupported column type: %', ctype;
  END IF;

  -- add column
  EXECUTE format('ALTER TABLE %I.%I ADD COLUMN IF NOT EXISTS %I %s %s',
                 sname, tname, cname, sql_type, CASE WHEN required THEN 'NOT NULL' ELSE '' END);

  -- create index when requested
  IF is_indexed THEN
    IF ctype = 'text' THEN
      EXECUTE format('CREATE INDEX IF NOT EXISTS %I ON %I.%I USING GIN (%I gin_trgm_ops)',
                     'ix_'||tname||'_'||cname||'_gin', sname, tname, cname);
    ELSE
      EXECUTE format('CREATE INDEX IF NOT EXISTS %I ON %I.%I (%I)',
                     'ix_'||tname||'_'||cname, sname, tname, cname);
    END IF;
  END IF;

  -- Add FK for UUID references when reference_table_id is set
  IF ctype = 'uuid' AND is_ref AND ref_table_id IS NOT NULL THEN
    -- ensure target table exists
    PERFORM app.ensure_physical_table(ref_table_id);
    SELECT COALESCE(schema_name,'app_data'), COALESCE(physical_table_name, 't_'||ref_table_id::text)
      INTO ref_sname, ref_tname
    FROM app.tables
    WHERE id = ref_table_id;

    fk_name := format('fk_%s_%s__%s', tname, cname, ref_tname);
    BEGIN
      EXECUTE format('ALTER TABLE %I.%I ADD CONSTRAINT %I FOREIGN KEY (%I) REFERENCES %I.%I(id) ON DELETE RESTRICT',
                     sname, tname, fk_name, cname, ref_sname, ref_tname);
    EXCEPTION WHEN duplicate_object THEN
      -- already exists
      NULL;
    END;

    -- optional: prevent self-reference to same row if referencing same table and require_different_table is set
    IF req_diff AND ref_table_id = t_id THEN
      BEGIN
        EXECUTE format('ALTER TABLE %I.%I ADD CONSTRAINT %I CHECK (%I IS NULL OR %I <> id)',
                       sname, tname, 'ck_'||tname||'_'||cname||'_not_self', cname, cname);
      EXCEPTION WHEN duplicate_object THEN NULL; END;
    END IF;
  END IF;
END;
$$;

COMMIT;
