-- Insert a sample Work Order

BEGIN;

WITH wo AS (SELECT id FROM app.tables WHERE slug = 'work_orders')
SELECT app.insert_row(
  (SELECT id FROM wo),
  jsonb_build_object(
    'title','Replace air filter on AHU-3',
    'description','Filter shows >200Pa ΔP. Swap and log reading.',
    'priority','MEDIUM',
    'status','OPEN',
    'estimated_duration_hours', 1.5,
    'required_signature', false,
    'archived', false
  )
) AS work_order_uuid;

COMMIT;