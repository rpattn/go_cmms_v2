BEGIN;

DROP FUNCTION IF EXISTS app.backfill_physical_all();
DROP FUNCTION IF EXISTS app.backfill_physical_for_table(bigint);

COMMIT;

