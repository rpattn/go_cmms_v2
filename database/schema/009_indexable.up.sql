CREATE OR REPLACE FUNCTION app.ensure_index(p_column_id bigint)
RETURNS void LANGUAGE plpgsql AS $$
DECLARE
  t app.column_type;
  idxname text;
  create_sql text;
  already_exists boolean;
BEGIN
  SELECT type INTO t FROM app.columns WHERE id = p_column_id;
  IF t IS NULL THEN RAISE EXCEPTION 'Unknown column_id %', p_column_id; END IF;

  IF t = 'text' THEN
    idxname := format('ix_text_%s', p_column_id);
    create_sql := format('CREATE INDEX %I ON app.values_text USING gin (value gin_trgm_ops) WHERE column_id = %L', idxname, p_column_id);
  ELSIF t = 'date' THEN
    idxname := format('ix_date_%s', p_column_id);
    create_sql := format('CREATE INDEX %I ON app.values_date (value) WHERE column_id = %L', idxname, p_column_id);
  ELSIF t = 'bool' THEN
    idxname := format('ix_bool_%s', p_column_id);
    create_sql := format('CREATE INDEX %I ON app.values_bool (value) WHERE column_id = %L', idxname, p_column_id);
  ELSIF t = 'enum' THEN
    idxname := format('ix_enum_%s', p_column_id);
    create_sql := format('CREATE INDEX %I ON app.values_enum (value) WHERE column_id = %L', idxname, p_column_id);
  ELSIF t = 'uuid' THEN
    idxname := format('ix_uuid_%s', p_column_id);
    create_sql := format('CREATE INDEX %I ON app.values_uuid (value) WHERE column_id = %L', idxname, p_column_id);
  END IF;
  
  IF idxname IS NULL THEN
    RETURN;
  END IF;

  SELECT EXISTS (
    SELECT 1 FROM pg_indexes WHERE schemaname = 'app' AND indexname = idxname
  )
  INTO already_exists;

  IF already_exists THEN
    RETURN;
  END IF;

  PERFORM pg_advisory_xact_lock(p_column_id);

  SELECT EXISTS (
    SELECT 1 FROM pg_indexes WHERE schemaname = 'app' AND indexname = idxname
  )
  INTO already_exists;

  IF already_exists THEN
    RETURN;
  END IF;

  EXECUTE create_sql;
END$$;


-- Optional

CREATE OR REPLACE FUNCTION app.on_columns_change()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.is_indexed AND (OLD.is_indexed IS DISTINCT FROM NEW.is_indexed) THEN
    PERFORM app.ensure_index(NEW.id);
  END IF;
  RETURN NEW;
END$$;

DROP TRIGGER IF EXISTS trg_columns_index ON app.columns;
CREATE TRIGGER trg_columns_index
AFTER INSERT OR UPDATE ON app.columns
FOR EACH ROW EXECUTE FUNCTION app.on_columns_change();
