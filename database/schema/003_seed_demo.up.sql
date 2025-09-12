BEGIN;

-- ================== CONFIG (edit these) ==================
CREATE TEMP TABLE _seed_cfg (
  email     text,
  name      text,
  username  text,
  phc       text,
  org_slug  text,
  org_name  text,
  ms_tenant text
) ON COMMIT DROP;

INSERT INTO _seed_cfg (email, name, username, phc, org_slug, org_name, ms_tenant)
VALUES (
  'admin@example.com',
  'Admin',
  'admin@example.com',
  '$argon2id$v=19$m=65536,t=3,p=1$7pTK54VYY7R7gT2NkvX6QQ$dxm4Q3bnBQXtwXpgMmmL8uQkxke/19Iz+yRVi8GQcT8',
  'acme',
  'Acme Inc.',
  NULL
);

-- Optional: seed multiple orgs here
CREATE TEMP TABLE _seed_orgs (
  slug text,
  name text,
  ms_tenant_id text
) ON COMMIT DROP;

INSERT INTO _seed_orgs (slug, name, ms_tenant_id) VALUES
  ('acme',    'Acme Inc.',     NULL),
  ('north-shore-wind', 'North Shore Wind Farm', NULL),
  ('testOrg', 'Test Org Inc.', NULL);
-- =========================================================

-- Upsert organisations
INSERT INTO organisations (slug, name, ms_tenant_id)
SELECT slug, name, ms_tenant_id FROM _seed_orgs
ON CONFLICT (slug) DO UPDATE
  SET name = EXCLUDED.name,
      ms_tenant_id = EXCLUDED.ms_tenant_id;

-- Upsert the user
INSERT INTO users (email, name)
SELECT email, name FROM _seed_cfg
ON CONFLICT (email) DO UPDATE
  SET name = EXCLUDED.name;

-- Upsert local credentials
INSERT INTO local_credentials (user_id, username, password_hash)
SELECT u.id, c.username, c.phc
FROM _seed_cfg c
JOIN users u ON u.email = c.email
ON CONFLICT (user_id) DO UPDATE
  SET password_hash = EXCLUDED.password_hash,
      last_password_change = now();

-- Add a 'local' identity (no-op if subject already taken)
INSERT INTO identities (user_id, provider, subject)
SELECT u.id, 'local', c.username
FROM _seed_cfg c
JOIN users u ON u.email = c.email
ON CONFLICT (provider, subject) DO NOTHING;

-- Ensure we can upsert memberships cleanly
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM   pg_indexes
    WHERE  schemaname = 'public'
      AND  tablename  = 'org_memberships'
      AND  indexname  = 'org_memberships_user_org_key'
  ) THEN
    EXECUTE 'CREATE UNIQUE INDEX org_memberships_user_org_key ON org_memberships(user_id, org_id)';
  END IF;
END $$;

-- Upsert membership as 'Owner'
INSERT INTO org_memberships (user_id, org_id, role)
SELECT u.id, o.id, 'Owner'
FROM _seed_cfg c
JOIN users u ON u.email = c.email
JOIN organisations o ON o.slug = c.org_slug
ON CONFLICT (user_id, org_id) DO UPDATE
  SET role = EXCLUDED.role;


-- ================================== Second user =========================================

INSERT INTO _seed_cfg (email, name, username, phc, org_slug, org_name, ms_tenant)
VALUES (
  'admin@nsw.com',
  'Admin',
  'admin@nsw.com',
  '$argon2id$v=19$m=65536,t=3,p=1$7pTK54VYY7R7gT2NkvX6QQ$dxm4Q3bnBQXtwXpgMmmL8uQkxke/19Iz+yRVi8GQcT8',
  'north-shore-wind',
  'North Shore Wind Farm',
  NULL
);

-- =========================================================

-- Upsert organisations
INSERT INTO organisations (slug, name, ms_tenant_id)
SELECT slug, name, ms_tenant_id FROM _seed_orgs
ON CONFLICT (slug) DO UPDATE
  SET name = EXCLUDED.name,
      ms_tenant_id = EXCLUDED.ms_tenant_id;

-- Upsert the user
INSERT INTO users (email, name)
SELECT email, name FROM _seed_cfg
ON CONFLICT (email) DO UPDATE
  SET name = EXCLUDED.name;

-- Upsert local credentials
INSERT INTO local_credentials (user_id, username, password_hash)
SELECT u.id, c.username, c.phc
FROM _seed_cfg c
JOIN users u ON u.email = c.email
ON CONFLICT (user_id) DO UPDATE
  SET password_hash = EXCLUDED.password_hash,
      last_password_change = now();

-- Add a 'local' identity (no-op if subject already taken)
INSERT INTO identities (user_id, provider, subject)
SELECT u.id, 'local', c.username
FROM _seed_cfg c
JOIN users u ON u.email = c.email
ON CONFLICT (provider, subject) DO NOTHING;

-- Ensure we can upsert memberships cleanly
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM   pg_indexes
    WHERE  schemaname = 'public'
      AND  tablename  = 'org_memberships'
      AND  indexname  = 'org_memberships_user_org_key'
  ) THEN
    EXECUTE 'CREATE UNIQUE INDEX org_memberships_user_org_key ON org_memberships(user_id, org_id)';
  END IF;
END $$;

-- Upsert membership as 'Owner'
INSERT INTO org_memberships (user_id, org_id, role)
SELECT u.id, o.id, 'Owner'
FROM _seed_cfg c
JOIN users u ON u.email = c.email
JOIN organisations o ON o.slug = c.org_slug
ON CONFLICT (user_id, org_id) DO UPDATE
  SET role = EXCLUDED.role;

COMMIT;
