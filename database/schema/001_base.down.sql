-- Drop indexes first
DROP INDEX IF EXISTS identities_user_id_idx;
DROP INDEX IF EXISTS org_memberships_user_id_idx;

-- Drop tables that depend on others first
DROP TABLE IF EXISTS idp_group_role_mappings;
DROP TABLE IF EXISTS org_memberships;
DROP TABLE IF EXISTS organisations;
DROP TABLE IF EXISTS identities;
DROP TABLE IF EXISTS users;

-- Drop enum type last
DROP TYPE IF EXISTS org_role;

-- Optionally drop the extension (careful in shared DBs)
DROP EXTENSION IF EXISTS "uuid-ossp";
