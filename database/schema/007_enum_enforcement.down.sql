-- DOWN migration for app.enforce_enum() and its trigger

-- Drop the trigger that uses the function
DROP TRIGGER IF EXISTS trg_values_enum_check ON app.values_enum;

-- Drop the function itself
DROP FUNCTION IF EXISTS app.enforce_enum() CASCADE;
