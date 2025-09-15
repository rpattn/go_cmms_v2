-- Org scoping (phase 1): add org_id to app.tables and scope search by org
-- This phase keeps rows org-less; queries are updated to filter table by org.

BEGIN;

-- 1) Add org_id to app.tables (nullable for backfill), FK to organisations
ALTER TABLE app.tables
  ADD COLUMN IF NOT EXISTS org_id uuid REFERENCES organisations(id);

-- 2) Drop global unique on slug and replace with per-org uniqueness
DO $$ BEGIN
  ALTER TABLE app.tables DROP CONSTRAINT IF EXISTS tables_slug_key;
EXCEPTION WHEN undefined_object THEN NULL; END $$;

-- Unique (org_id, slug) per tenant. Allows multiple orgs to reuse the same slug.
CREATE UNIQUE INDEX IF NOT EXISTS app_tables_org_slug_uniq ON app.tables(org_id, slug);

-- Optional: also ensure human names are unique per org case-insensitively
CREATE UNIQUE INDEX IF NOT EXISTS app_tables_org_lower_name_uniq ON app.tables(org_id, lower(name));

COMMIT;

