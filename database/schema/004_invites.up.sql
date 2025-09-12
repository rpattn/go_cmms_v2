CREATE TABLE org_invites (
  token_hash TEXT PRIMARY KEY,
  org_id UUID NOT NULL REFERENCES organisations(id) ON DELETE CASCADE,
  email TEXT NOT NULL,
  role org_role NOT NULL DEFAULT 'Member',
  inviter_id UUID NOT NULL REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at TIMESTAMPTZ NOT NULL,
  used_at TIMESTAMPTZ
);

CREATE INDEX org_invites_org_id_idx ON org_invites (org_id);
CREATE INDEX org_invites_email_idx ON org_invites (email);
CREATE INDEX org_invites_expires_idx ON org_invites (expires_at);

