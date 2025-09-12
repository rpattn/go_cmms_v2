-- DOWN MIGRATION ONLY for Work Orders in JSONB EAV model
-- Rolls back objects created by the corresponding UP migration.
-- It:
--   1) Drops the relational counter table `work_order_counters`
--   2) Deletes the three M2M link logical tables (and their columns)
--   3) Deletes the `work_orders` logical table (and its columns)
-- NOTE: Does NOT drop referenced base logical tables (organisations, users, etc.).

BEGIN;

-- 1) Drop counter table (if it exists)
DROP TABLE IF EXISTS work_order_counters;

-- 2) Drop M2M link logical tables
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM app.tables WHERE slug = 'work_order_files') THEN
    DELETE FROM app.columns WHERE table_id = (SELECT id FROM app.tables WHERE slug='work_order_files');
    DELETE FROM app.tables  WHERE slug='work_order_files';
  END IF;

  IF EXISTS (SELECT 1 FROM app.tables WHERE slug = 'work_order_customers') THEN
    DELETE FROM app.columns WHERE table_id = (SELECT id FROM app.tables WHERE slug='work_order_customers');
    DELETE FROM app.tables  WHERE slug='work_order_customers';
  END IF;

  IF EXISTS (SELECT 1 FROM app.tables WHERE slug = 'work_order_assigned_to') THEN
    DELETE FROM app.columns WHERE table_id = (SELECT id FROM app.tables WHERE slug='work_order_assigned_to');
    DELETE FROM app.tables  WHERE slug='work_order_assigned_to';
  END IF;
END $$;

-- 3) Drop Work Orders logical table
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM app.tables WHERE slug = 'work_orders') THEN
    DELETE FROM app.columns WHERE table_id = (SELECT id FROM app.tables WHERE slug='work_orders');
    DELETE FROM app.tables  WHERE slug='work_orders';
  END IF;
END $$;

COMMIT;