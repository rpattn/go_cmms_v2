CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE users (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  email TEXT UNIQUE NOT NULL,
  name TEXT,
  avatar_url TEXT,
  phone TEXT,
  country TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE identities (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  provider TEXT NOT NULL,
  subject TEXT NOT NULL,
  UNIQUE (provider, subject)
);

CREATE TABLE organisations (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  slug TEXT UNIQUE NOT NULL,
  name TEXT NOT NULL,
  ms_tenant_id TEXT,  -- nullable; fill if you map Microsoft tenant->org
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (ms_tenant_id)
);

DO $$ BEGIN
  CREATE TYPE org_role AS ENUM ('Owner','Admin','Member','Viewer');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE org_memberships (
  org_id UUID NOT NULL REFERENCES organisations(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role org_role NOT NULL DEFAULT 'Member',
  PRIMARY KEY (org_id, user_id)
);

-- Optional group->role mapping (enterprise)
CREATE TABLE idp_group_role_mappings (
  org_id UUID NOT NULL REFERENCES organisations(id) ON DELETE CASCADE,
  provider TEXT NOT NULL,
  idp_group_id TEXT NOT NULL,
  role_name org_role NOT NULL,
  PRIMARY KEY (org_id, provider, idp_group_id)
);

CREATE INDEX ON identities (user_id);
CREATE INDEX ON org_memberships (user_id);

-- Optional but recommended for fast ILIKE '%...%' searches
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- GIN trigram indexes to speed up cn/sw/ew on email and name
CREATE INDEX IF NOT EXISTS users_email_trgm_idx ON users USING gin (email gin_trgm_ops);
CREATE INDEX IF NOT EXISTS users_name_trgm_idx  ON users USING gin (name  gin_trgm_ops);
