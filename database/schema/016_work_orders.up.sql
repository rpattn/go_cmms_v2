-- Work Orders in JSONB EAV model (PostgreSQL)
-- Assumes your EAV metadata lives in app.tables / app.columns
-- and you have helper fns like app.insert_row(jsonb) similar to your example.
--
-- This migration:
-- 1) Upserts the Work Orders logical table
-- 2) Declares columns mirroring the relational WorkOrder schema
-- 3) Declares M2M link tables (Assigned Users, Customers, Files)
-- 4) Adds a per-org, per-year counter table for custom_id generation (relational)
-- 5) Shows a tiny seed example

BEGIN;

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ---------------------------------------------------------------
-- Ensure referenced logical tables exist (so we can reference them)
-- ---------------------------------------------------------------
WITH upserts AS (
  INSERT INTO app.tables (name, slug)
  VALUES
    ('Organisations','organisations'),
    ('Users','users'),
    ('Files','files'),
    ('Requests','requests'),
    ('Preventive Maintenances','preventive_maintenances'),
    ('Work Order Categories','work_order_categories'),
    ('Locations','locations'),
    ('Teams','teams'),
    ('Customers','customers'),
    ('Assets','assets')
  ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name
  RETURNING id, slug
)
SELECT 1;

-- ---------------------------------------------------------------
-- Create/Upsert Work Orders logical table
-- ---------------------------------------------------------------
WITH wo AS (
  INSERT INTO app.tables (name, slug)
  VALUES ('Work Orders','work_orders')
  ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name
  RETURNING id
),
refs AS (
  SELECT
    (SELECT id FROM app.tables WHERE slug = 'organisations')   AS organisations_id,
    (SELECT id FROM app.tables WHERE slug = 'users')           AS users_id,
    (SELECT id FROM app.tables WHERE slug = 'files')           AS files_id,
    (SELECT id FROM app.tables WHERE slug = 'requests')        AS requests_id,
    (SELECT id FROM app.tables WHERE slug = 'preventive_maintenances') AS pm_id,
    (SELECT id FROM app.tables WHERE slug = 'work_order_categories')   AS categories_id,
    (SELECT id FROM app.tables WHERE slug = 'locations')       AS locations_id,
    (SELECT id FROM app.tables WHERE slug = 'teams')           AS teams_id,
    (SELECT id FROM app.tables WHERE slug = 'customers')       AS customers_id,
    (SELECT id FROM app.tables WHERE slug = 'assets')          AS assets_id
)
INSERT INTO app.columns
  (table_id, name,                        type,                       is_required, is_indexed, is_reference, reference_table_id, require_different_table, enum_values)
SELECT wo.id, 'title',                    'text'::app.column_type,    true,        true,       false,        NULL::bigint,       false,                   NULL::text[] FROM wo
UNION ALL
SELECT wo.id, 'description',              'text'::app.column_type,    false,       false,      false,        NULL::bigint,       false,                   NULL::text[] FROM wo
UNION ALL
SELECT wo.id, 'priority',                 'enum'::app.column_type,    false,       true,       false,        NULL::bigint,       false,                   ARRAY['NONE','LOW','MEDIUM','HIGH','URGENT']::text[] FROM wo
UNION ALL
SELECT wo.id, 'status',                   'enum'::app.column_type,    false,       true,       false,        NULL::bigint,       false,                   ARRAY['OPEN','IN_PROGRESS','ON_HOLD','COMPLETED','CANCELLED']::text[] FROM wo
UNION ALL
SELECT wo.id, 'required_signature',       'bool'::app.column_type,    false,       true,       false,        NULL::bigint,       false,                   NULL::text[] FROM wo
UNION ALL
SELECT wo.id, 'archived',                 'bool'::app.column_type,    false,       true,       false,        NULL::bigint,       false,                   NULL::text[] FROM wo
UNION ALL
SELECT wo.id, 'custom_id',                'text'::app.column_type,    false,       true,       false,        NULL::bigint,       false,                   NULL::text[] FROM wo
UNION ALL
SELECT wo.id, 'estimated_duration_hours', 'float'::app.column_type,   false,       false,      false,        NULL::bigint,       false,                   NULL::text[] FROM wo
UNION ALL
SELECT wo.id, 'due_date',                 'date'::app.column_type,false,       true,       false,        NULL::bigint,       false,                   NULL::text[] FROM wo
UNION ALL
SELECT wo.id, 'estimated_start_date',     'date'::app.column_type,false,       false,      false,        NULL::bigint,       false,                   NULL::text[] FROM wo
UNION ALL
SELECT wo.id, 'completed_on',             'date'::app.column_type,false,       true,       false,        NULL::bigint,       false,                   NULL::text[] FROM wo
UNION ALL
SELECT wo.id, 'first_time_to_react',      'date'::app.column_type,false,       false,      false,        NULL::bigint,       false,                   NULL::text[] FROM wo
-- References (singular)
UNION ALL SELECT wo.id, 'organisation',    'uuid'::app.column_type,    false,       true,       true,         (SELECT organisations_id FROM refs), false, NULL::text[] FROM wo, refs
UNION ALL SELECT wo.id, 'created_by',      'uuid'::app.column_type,    false,       true,       true,         (SELECT users_id FROM refs),         true,  NULL::text[] FROM wo, refs
UNION ALL SELECT wo.id, 'primary_user',    'uuid'::app.column_type,    false,       true,       true,         (SELECT users_id FROM refs),         true,  NULL::text[] FROM wo, refs
UNION ALL SELECT wo.id, 'completed_by',    'uuid'::app.column_type,    false,       true,       true,         (SELECT users_id FROM refs),         true,  NULL::text[] FROM wo, refs
UNION ALL SELECT wo.id, 'image',           'uuid'::app.column_type,    false,       false,      true,         (SELECT files_id FROM refs),         true,  NULL::text[] FROM wo, refs
UNION ALL SELECT wo.id, 'signature_file',  'uuid'::app.column_type,    false,       false,      true,         (SELECT files_id FROM refs),         true,  NULL::text[] FROM wo, refs
UNION ALL SELECT wo.id, 'category',        'uuid'::app.column_type,    false,       true,       true,         (SELECT categories_id FROM refs),    true,  NULL::text[] FROM wo, refs
UNION ALL SELECT wo.id, 'location',        'uuid'::app.column_type,    false,       true,       true,         (SELECT locations_id FROM refs),     true,  NULL::text[] FROM wo, refs
UNION ALL SELECT wo.id, 'team',            'uuid'::app.column_type,    false,       true,       true,         (SELECT teams_id FROM refs),         true,  NULL::text[] FROM wo, refs
UNION ALL SELECT wo.id, 'asset',           'uuid'::app.column_type,    false,       true,       true,         (SELECT assets_id FROM refs),        true,  NULL::text[] FROM wo, refs
UNION ALL SELECT wo.id, 'parent_request',  'uuid'::app.column_type,    false,       true,       true,         (SELECT requests_id FROM refs),      true,  NULL::text[] FROM wo, refs
UNION ALL SELECT wo.id, 'parent_pm',       'uuid'::app.column_type,    false,       true,       true,         (SELECT pm_id FROM refs),            true,  NULL::text[] FROM wo, refs
-- Feedback free text
UNION ALL
SELECT wo.id, 'feedback',                 'text'::app.column_type,    false,       false,      false,        NULL::bigint,       false,                   NULL::text[] FROM wo
ON CONFLICT (table_id, name) DO NOTHING;

-- ---------------------------------------------------------------
-- M2M link tables in EAV style (one row per link)
-- ---------------------------------------------------------------
WITH wo AS (SELECT id FROM app.tables WHERE slug = 'work_orders'),
     users AS (SELECT id FROM app.tables WHERE slug = 'users'),
     customers AS (SELECT id FROM app.tables WHERE slug = 'customers'),
     files AS (SELECT id FROM app.tables WHERE slug = 'files')
-- Assigned Users
INSERT INTO app.tables (name, slug)
VALUES ('Work Order ↔ Assigned User','work_order_assigned_to')
ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name;

WITH link AS (SELECT id FROM app.tables WHERE slug = 'work_order_assigned_to'),
     wo AS (SELECT id FROM app.tables WHERE slug = 'work_orders'),
     u AS (SELECT id FROM app.tables WHERE slug = 'users')
INSERT INTO app.columns (table_id, name, type, is_required, is_indexed, is_reference, reference_table_id, require_different_table, enum_values)
SELECT (SELECT id FROM link), 'work_order', 'uuid'::app.column_type, true, true, true, (SELECT id FROM wo), true, NULL::text[]
UNION ALL
SELECT (SELECT id FROM link), 'user',       'uuid'::app.column_type, true, true, true, (SELECT id FROM u),  true, NULL::text[]
ON CONFLICT (table_id, name) DO NOTHING;

-- Customers on a Work Order
INSERT INTO app.tables (name, slug)
VALUES ('Work Order ↔ Customer','work_order_customers')
ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name;

WITH link AS (SELECT id FROM app.tables WHERE slug = 'work_order_customers'),
     wo AS (SELECT id FROM app.tables WHERE slug = 'work_orders'),
     c  AS (SELECT id FROM app.tables WHERE slug = 'customers')
INSERT INTO app.columns (table_id, name, type, is_required, is_indexed, is_reference, reference_table_id, require_different_table, enum_values)
SELECT (SELECT id FROM link), 'work_order', 'uuid'::app.column_type, true, true, true, (SELECT id FROM wo), true, NULL::text[]
UNION ALL
SELECT (SELECT id FROM link), 'customer',   'uuid'::app.column_type, true, true, true, (SELECT id FROM c),  true, NULL::text[]
ON CONFLICT (table_id, name) DO NOTHING;

-- Files attached to a Work Order
INSERT INTO app.tables (name, slug)
VALUES ('Work Order ↔ File','work_order_files')
ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name;

WITH link AS (SELECT id FROM app.tables WHERE slug = 'work_order_files'),
     wo AS (SELECT id FROM app.tables WHERE slug = 'work_orders'),
     f  AS (SELECT id FROM app.tables WHERE slug = 'files')
INSERT INTO app.columns (table_id, name, type, is_required, is_indexed, is_reference, reference_table_id, require_different_table, enum_values)
SELECT (SELECT id FROM link), 'work_order', 'uuid'::app.column_type, true, true, true, (SELECT id FROM wo), true, NULL::text[]
UNION ALL
SELECT (SELECT id FROM link), 'file',       'uuid'::app.column_type, true, true, true, (SELECT id FROM f),  true, NULL::text[]
ON CONFLICT (table_id, name) DO NOTHING;

-- ---------------------------------------------------------------
-- (Optional) Relational counter table for custom_id generation
-- Mirrors the per-org, per-year counters in your original schema
-- ---------------------------------------------------------------
CREATE TABLE IF NOT EXISTS work_order_counters (
  organisation_id UUID NOT NULL,
  year            INTEGER NOT NULL,
  next_seq        INTEGER NOT NULL DEFAULT 1,
  PRIMARY KEY (organisation_id, year)
);

COMMIT;
-- ---------------------------------------------------------------

