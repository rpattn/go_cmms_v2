-- Drop indexes created on login_attempts
DROP INDEX IF EXISTS login_attempts_username_ts_idx;
DROP INDEX IF EXISTS login_attempts_ip_ts_idx;

-- Drop tables created by the migration
DROP TABLE IF EXISTS local_credentials;
DROP TABLE IF EXISTS user_totp;
DROP TABLE IF EXISTS password_resets;
DROP TABLE IF EXISTS login_attempts;
