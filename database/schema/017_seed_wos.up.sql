-- Insert sample Work Orders for North Shore Wind

BEGIN;

-- Seed additional Work Orders for org "north-shore-wind"
WITH wo AS (SELECT id FROM app.tables WHERE slug = 'work_orders')
SELECT app.insert_row((SELECT id FROM wo), jsonb_build_object(
  'title','Replace air filter on AHU-3',
  'description','[NSW] Filter ΔP >200Pa. Swap and log reading.',
  'priority','MEDIUM',
  'status','OPEN',
  'estimated_duration_hours', 1.5,
  'required_signature', false,
  'archived', false
));

WITH wo AS (SELECT id FROM app.tables WHERE slug = 'work_orders')
SELECT app.insert_row((SELECT id FROM wo), jsonb_build_object(
  'title','Inspect Gearbox G-21',
  'description','[NSW] Routine inspection and oil sample.',
  'priority','HIGH',
  'status','IN_PROGRESS',
  'estimated_duration_hours', 3.0,
  'required_signature', false,
  'archived', false,
  'estimated_start_date','2025-01-12'
));

WITH wo AS (SELECT id FROM app.tables WHERE slug = 'work_orders')
SELECT app.insert_row((SELECT id FROM wo), jsonb_build_object(
  'title','Tighten tower bolts - T05',
  'description','[NSW] Torque check for tower section 2.',
  'priority','LOW',
  'status','OPEN',
  'estimated_duration_hours', 2.0,
  'required_signature', false,
  'archived', false,
  'due_date','2025-01-20'
));

WITH wo AS (SELECT id FROM app.tables WHERE slug = 'work_orders')
SELECT app.insert_row((SELECT id FROM wo), jsonb_build_object(
  'title','Calibrate anemometer A-7',
  'description','[NSW] Annual calibration; attach certificate.',
  'priority','MEDIUM',
  'status','ON_HOLD',
  'estimated_duration_hours', 1.0,
  'required_signature', true,
  'archived', false
));

WITH wo AS (SELECT id FROM app.tables WHERE slug = 'work_orders')
SELECT app.insert_row((SELECT id FROM wo), jsonb_build_object(
  'title','Replace yaw motor YM-2',
  'description','[NSW] Motor shows overheating alarms; replace and test.',
  'priority','URGENT',
  'status','OPEN',
  'estimated_duration_hours', 4.5,
  'required_signature', true,
  'archived', false
));

WITH wo AS (SELECT id FROM app.tables WHERE slug = 'work_orders')
SELECT app.insert_row((SELECT id FROM wo), jsonb_build_object(
  'title','Lubricate main bearing MB-1',
  'description','[NSW] PM task - use spec’d grease.',
  'priority','MEDIUM',
  'status','COMPLETED',
  'estimated_duration_hours', 2.5,
  'required_signature', false,
  'archived', false,
  'completed_on','2025-01-08'
));

WITH wo AS (SELECT id FROM app.tables WHERE slug = 'work_orders')
SELECT app.insert_row((SELECT id FROM wo), jsonb_build_object(
  'title','Inspect blade B2 (drone)',
  'description','[NSW] Visible crack near root; capture imagery.',
  'priority','HIGH',
  'status','OPEN',
  'estimated_duration_hours', 1.5,
  'required_signature', false,
  'archived', false
));

WITH wo AS (SELECT id FROM app.tables WHERE slug = 'work_orders')
SELECT app.insert_row((SELECT id FROM wo), jsonb_build_object(
  'title','Check transformer TR-3 oil level',
  'description','[NSW] Low-level alarm last night; verify and top up.',
  'priority','HIGH',
  'status','IN_PROGRESS',
  'estimated_duration_hours', 1.0,
  'required_signature', false,
  'archived', false
));

WITH wo AS (SELECT id FROM app.tables WHERE slug = 'work_orders')
SELECT app.insert_row((SELECT id FROM wo), jsonb_build_object(
  'title','PM: Nacelle inspection - N07',
  'description','[NSW] Quarterly PM checklist.',
  'priority','MEDIUM',
  'status','OPEN',
  'estimated_duration_hours', 2.0,
  'required_signature', false,
  'archived', false
));

WITH wo AS (SELECT id FROM app.tables WHERE slug = 'work_orders')
SELECT app.insert_row((SELECT id FROM wo), jsonb_build_object(
  'title','Replace HVAC filter (tower base)',
  'description','[NSW] Increased dust; replace both pre and fine filters.',
  'priority','LOW',
  'status','OPEN',
  'estimated_duration_hours', 0.8,
  'required_signature', false,
  'archived', false
));

WITH wo AS (SELECT id FROM app.tables WHERE slug = 'work_orders')
SELECT app.insert_row((SELECT id FROM wo), jsonb_build_object(
  'title','Test emergency stop circuits',
  'description','[NSW] Annual safety verification.',
  'priority','HIGH',
  'status','OPEN',
  'estimated_duration_hours', 1.2,
  'required_signature', true,
  'archived', false
));

COMMIT;

