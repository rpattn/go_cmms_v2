# Authentication & Session Security Review

Date: 2025-09-07

This document records a targeted review of the authentication, session, and related database schema in this repository. It summarizes risks, affected code, and concrete remediation tasks.

## Executive Summary

Overall auth design is sound (opaque server-side sessions, Argon2id for local passwords, optional TOTP, OIDC providers). We identified several high- and medium-risk issues to address before production use.

## Recent Changes (Sep 2025)

- Signup requires a unique `org_slug`; creator becomes `Owner`. Existing slug returns 409.
- Provider sign-in no longer auto-creates users or auto-assigns an org when the email is unknown; redirects to `/account/register` (or returns 409 `signup_required`). Existing email without link uses explicit link flow (no auto-link).
- Invites implemented: Owners create invites, recipients accept and get a session, then set password; static helper pages provided under `/invite` and `/invite/accept`.
- Added `frontend.api_route` (optional) for frontend proxying; URLs are normalized and avoid double slashes.

## High-Risk Findings

- Organisation access at signup
  - Previous behavior allowed self-joining existing orgs by slug.
  - Impact: Potential unauthorized access by guessing/knowing slugs.
  - Mitigation (implemented): Signup now requires a unique `org_slug`; if it already exists, signup is rejected with 409. When unique, a new organisation is created and the creator is assigned `Owner`.
  - Code (after change): `internal/auth/local.go` (SignupHandler creates org and sets Owner), `internal/repo/repo.go` + `internal/repo/orgs.go` (CreateOrg).

- Fallback redirect trusts request headers (open redirect)
  - When `frontend.url` is unset, the OAuth callback redirects using `X-Forwarded-Proto` and `X-Forwarded-Host`.
  - Code: `internal/auth/handlers.go:204`, `internal/auth/handlers.go:209`.
  - Impact: Attacker-controlled redirect target; phishing or cookie misdirection.
  - Fix: Always redirect to a configured allowlisted base URL; do not derive from request headers.

- Session IDs exposed in admin endpoint
  - Admin session list returns the raw session ID (bearer token).
  - Code: `internal/handlers/admin/admin.go:14`, `internal/handlers/admin/admin.go:38`.
  - Impact: Increased risk of lateral movement if IDs leak via logs or tooling.
  - Fix: Omit or hash/truncate session IDs in responses; restrict visibility.

## Moderate-Risk Findings

- Cookie security defaults
  - Config default sets `security.session.cookie_secure=false` (dev-friendly), but docs imply Secure always.
  - Code: `internal/config/config.go:75` (default), set at runtime in `cmd/server/main.go:38`.
  - Fix: Ensure production configs set `SESSION_COOKIE_SECURE=true`; if `SameSite=none`, Secure must be true.

- GitHub email trust is weak
  - Uses `/user` and may synthesize `login@users.noreply.github.com`; does not check verified emails.
  - Code: `internal/auth/handlers.go:369` and `internal/auth/handlers.go:392` (GitHub branch).
  - Fix: Call `/user/emails` and require a primary, verified email; otherwise prompt to add local credential.

- Missing per-username lockout/throttle
  - DB records attempts and includes a `CountRecentFailures` query, but it is not used to enforce lockout.
  - Code: query in `database/queries/login_attempts.sql`, usage missing in local login flow.
  - Fix: Apply sliding-window throttling on username and IP with exponential backoff or temporary lockout.

- TOTP secret stored in plaintext
  - Schema stores `user_totp.secret` as plaintext.
  - Code: `database/schema/002_local_auth.up.sql:15`.
  - Fix: Encrypt at rest with an app key (KMS/HSM if available); rotate keys.

## Low-Risk / Observations

- Argon2id cost parameters are conservative (m=64MiB, t=1, p=4).
  - Code: `internal/auth/local.go:246` (`defaultArgonParams`).
  - Recommendation: Calibrate and consider `t=2..3` based on servers.

- In-memory session store (single-node)
  - Code: `internal/session/store.go`.
  - Note: Not HA; sessions lost on restart or other nodes. Consider Redis/DB-backed sessions for multi-instance.

## Remediation TODOs

- Signup/invite flow
  - Keep current “create org on signup” with Owner role for unique slugs.
  - Add invitation endpoints so existing Owners can invite users to an existing organisation (required path when `org_slug` exists).

- Redirect safety
  - Require `frontend.url` in production; remove header-derived fallback in `internal/auth/handlers.go:204`–`internal/auth/handlers.go:211`.
  - Validate any `redirect` query params against an allowlist.

- Session list hygiene
  - In `internal/handlers/admin/admin.go`, replace `ID` with a short, non-replayable reference or a truncated hash.

- Login throttling
  - Use `CountRecentFailures` to deny or delay after N failures per username/IP (e.g., 5 in 15 minutes).
  - Return 429/401 with generic error; continue recording attempts.

- GitHub email verification
  - Fetch `/user/emails`, pick primary verified; if none, block login or require link to existing verified email.

- TOTP at rest
  - Encrypt `user_totp.secret` with environment-provided key; add key rotation plan.

- Argon2 tuning
  - Benchmark and increase `Time` cost; consider memory increase if feasible.

- Proxy trust & client IP
  - Gate `X-Forwarded-*` parsing behind a trusted proxy setting; otherwise ignore.

## Deployment Notes

- Set `SESSION_COOKIE_SECURE=true` and prefer `SameSite=Lax` unless cross-site needs are explicit.
- Set `FRONTEND_URL` (and `frontend.post_login_path`) to avoid unsafe fallbacks.
- Keep rate limiting enabled; consider separate stricter limits for auth endpoints.

## Affected Files (reference)

- `internal/auth/local.go:32`
- `internal/auth/local.go:71`
- `internal/auth/handlers.go:204`
- `internal/handlers/admin/admin.go:14`
- `internal/config/config.go:75`
- `database/schema/002_local_auth.up.sql:15`
- `internal/auth/local.go:246`
