BEGIN;

-- No safe automatic rollback: restore previous function if you had one
DROP FUNCTION IF EXISTS app.row_to_json(uuid);

COMMIT;

