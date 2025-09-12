CREATE TABLE local_credentials (
  user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  username TEXT UNIQUE NOT NULL,
  password_hash TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_password_change TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE user_totp (
  user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  secret TEXT NOT NULL,
  issuer TEXT NOT NULL,
  label TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE password_resets (
  token TEXT PRIMARY KEY,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  expires_at TIMESTAMPTZ NOT NULL,
  used_at TIMESTAMPTZ
);

CREATE TABLE login_attempts (
  id BIGSERIAL PRIMARY KEY,
  username TEXT NOT NULL,
  ip INET NOT NULL,
  ts TIMESTAMPTZ NOT NULL DEFAULT now(),
  success BOOLEAN NOT NULL
);
CREATE INDEX login_attempts_username_ts_idx ON login_attempts (username, ts);
CREATE INDEX login_attempts_ip_ts_idx ON login_attempts (ip, ts);
