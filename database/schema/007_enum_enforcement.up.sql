CREATE OR REPLACE FUNCTION app.enforce_enum()
RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
  allowed text[];
BEGIN
  SELECT c.enum_values INTO allowed FROM app.columns c WHERE c.id = NEW.column_id;
  IF allowed IS NULL THEN
    RAISE EXCEPTION 'Column % is not enum-typed', NEW.column_id;
  END IF;
  IF NEW.value IS NOT NULL AND NOT (NEW.value = ANY(allowed)) THEN
    RAISE EXCEPTION 'Enum value "%" not in % for column %', NEW.value, allowed, NEW.column_id;
  END IF;
  RETURN NEW;
END$$;

DROP TRIGGER IF EXISTS trg_values_enum_check ON app.values_enum;
CREATE TRIGGER trg_values_enum_check
BEFORE INSERT OR UPDATE ON app.values_enum
FOR EACH ROW EXECUTE FUNCTION app.enforce_enum();
