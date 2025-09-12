-- DOWN migration for app.enforce_uuid_reference() and its trigger

-- Drop the trigger first (depends on the function + table)
DROP TRIGGER IF EXISTS trg_values_uuid_check ON app.values_uuid;

-- Then drop the function
DROP FUNCTION IF EXISTS app.enforce_uuid_reference() CASCADE;
