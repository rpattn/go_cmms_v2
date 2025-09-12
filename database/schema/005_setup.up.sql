-- Recommended on Postgres 13+ for gen_random_uuid()
CREATE EXTENSION IF NOT EXISTS pgcrypto;
-- For fast text search & LIKE/ILIKE
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- Keep things tidy
CREATE SCHEMA IF NOT EXISTS app;
SET search_path = app, public;
