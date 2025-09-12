-- DOWN migration
-- Reverts:
--   CREATE SCHEMA IF NOT EXISTS app;
--   CREATE EXTENSION IF NOT EXISTS pg_trgm;
--   CREATE EXTENSION IF NOT EXISTS pgcrypto;
-- Notes:
--   - SET search_path is session-scoped, so no down step needed.
--   - CASCADE will drop all objects inside schema app (tables, views, etc.).
--     If you don't want that, remove CASCADE and ensure the schema is empty first.

-- Drop the schema created for the app (and its contents).
DROP SCHEMA IF EXISTS app CASCADE;

-- Remove extensions (only if you truly want them gone cluster-wide).
--DROP EXTENSION IF EXISTS pg_trgm;
--DROP EXTENSION IF EXISTS pgcrypto;
