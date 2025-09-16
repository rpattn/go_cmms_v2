-- Rollback for dual-write helpers

BEGIN;

DROP FUNCTION IF EXISTS app.delete_row_physical(bigint, uuid, uuid);
DROP FUNCTION IF EXISTS app.insert_row_physical(bigint, uuid, uuid, jsonb);

COMMIT;

