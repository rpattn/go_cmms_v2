CREATE OR REPLACE FUNCTION app.enforce_uuid_reference()
RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
  col       app.columns;
  src_table bigint;
  tgt_table bigint;
BEGIN
  -- Allow NULLs; required-ness is handled elsewhere
  IF NEW.value IS NULL THEN
    RETURN NEW;
  END IF;

  SELECT * INTO col FROM app.columns WHERE id = NEW.column_id;
  IF col.type <> 'uuid' OR NOT col.is_reference THEN
    RETURN NEW;
  END IF;

  SELECT table_id INTO src_table FROM app.rows WHERE id = NEW.row_id;
  SELECT table_id INTO tgt_table FROM app.rows WHERE id = NEW.value;

  IF col.reference_table_id IS NOT NULL AND tgt_table <> col.reference_table_id THEN
    RAISE EXCEPTION
      'UUID reference must target table_id=%, but row % is in table_id=%',
      col.reference_table_id, NEW.value, tgt_table;
  END IF;

  IF col.require_different_table AND src_table = tgt_table THEN
    RAISE EXCEPTION 'UUID reference must target a different table (src=% = tgt=%)',
      src_table, tgt_table;
  END IF;

  RETURN NEW;
END$$;

DROP TRIGGER IF EXISTS trg_values_uuid_check ON app.values_uuid;
CREATE TRIGGER trg_values_uuid_check
BEFORE INSERT OR UPDATE ON app.values_uuid
FOR EACH ROW EXECUTE FUNCTION app.enforce_uuid_reference();
