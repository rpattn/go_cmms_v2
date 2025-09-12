# Auth Flow and Endpoints (Frontend Guide)

This document explains how to integrate the frontend with the server's authentication, sessions, and MFA (TOTP) flows. It includes endpoints, request/response shapes, and UX guidance.

## Config Highlights

- `logging.level`, `logging.format` (text|json)
- `security.request_id.trust_header` — reads `X-Request-ID` if true; a request id is returned in responses.
- `security.mfa.local_required` — require TOTP for local users across the app.
- `security.rate_limit.*` — RPM, burst, TTL; returns 429 when exceeded.
- `security.session.sweeper_interval` — background cleanup cadence (e.g., `5m`).
- `frontend.url` — public origin of the frontend (e.g., `https://app.example.com`).
- `frontend.api_route` — path prefix on the frontend that proxies to this backend (default empty). If set (e.g., `/api/backend`), the server normalizes it to start with `/` and have no trailing slash; example: `api/backend/` becomes `/api/backend`.

## Overview

- Two auth modes:
  - Local username/password (optional app‑side TOTP).
  - Social/OIDC sign‑in (Microsoft, Google, GitHub). App does not enforce TOTP after OIDC; rely on IdP MFA.
- Sessions are opaque: a `session` cookie holds only a random id; the server stores session data.
- If `security.mfa.local_required=true`, local users must enroll TOTP before using most routes.
- Request logging and rate limiting are enabled; responses may include standard HTTP errors (401, 403, 429).

## Config Highlights

- `logging.level`, `logging.format` (text|json)
- `security.request_id.trust_header` → reads `X-Request-ID` if true; a request id is returned in responses.
- `security.mfa.local_required` → require TOTP for local users across the app.
- `security.rate_limit.*` → RPM, burst, TTL; returns 429 when exceeded.
- `security.session.sweeper_interval` → background cleanup cadence (e.g., `5m`).

## Sessions

- Cookie name: `session` (HttpOnly, SameSite=Lax by default; `Secure` is configured via `security.session.cookie_secure` — ensure `true` in production. If you set `SameSite=none`, `Secure` must be `true`).
- Server maintains session data: `user_id`, `active_org`, `provider`, `expiry`.
- Logout clears cookie and server session.

### Endpoint: POST `/auth/logout`

- Request: send with credentials (cookie).
- Response: 204 No Content.

## Local Auth (username/password)

### Endpoint: POST `/auth/signup`

Body (org_slug is required, must be unique, and must match `^[a-z0-9](?:[a-z0-9-]{1,30}[a-z0-9])$`)

```json
{ "email": "user@example.com", "username": "user", "name": "User Name", "password": "****", "org_slug": "acme" }
```

Responses

- 201 Created on success; sets `session` cookie. Behavior:
  - Creates a new organisation with `slug=org_slug` and sets the signing-up user as `Owner`.
  - Sets the session with `active_org` to that newly created org.
- 400 if `org_slug` is missing or invalid.
- 409 `{ "error": "org_exists" }` if an organisation already exists with that slug. In that case, sign-up must proceed via an invite from an existing owner (invite flow TBD).

### Organisation Invites
### Organisation Invites

- POST `/auth/invite` (Owner required): create an invite for an email and optional role. Returns an acceptance URL built from `frontend.url` + optional `frontend.api_route`.
- POST `/auth/invite/accept`: accept an invite with a token. Creates/loads the user by invite email, adds membership, sets a session.
- POST `/auth/set-password`: sets/updates the local password for the current session (used immediately after acceptance). Responds with JSON containing a `redirect` URL (or issues a 303 when Accept includes `text/html`).

See `docs/invites.md` for details. Email delivery is app-specific.

- POST `/auth/invite` (Owner required): create an invite for an email and optional role.
- POST `/auth/invite/accept`: accept an invite with a token.

Note: endpoints are implemented; see `docs/invites.md` for details. Email delivery is app-specific.

### Endpoint: POST `/auth/login`

Body

```json
{ "username": "user", "password": "****", "totp_code": "123456" }
```

Behavior

- If user has TOTP enabled, `totp_code` is required; otherwise 401.
- If `security.mfa.local_required=true` and the user has no TOTP yet:
  - Login succeeds (session issued), but most routes are blocked until TOTP setup.
  - Browser requests get redirected to `/static/mfa/index.html` when hitting blocked routes.
  - API requests receive `403 {"error":"mfa_required"}`.

## TOTP (MFA) for Local Accounts

Used only for local users; not enforced for OIDC providers.

### Endpoint: GET `/auth/mfa/totp/setup`

- Requires a valid session cookie.
- Returns JSON with the secret and an `otpauth://` URL for authenticator apps.

Response

```json
{ "secret": "BASE32SECRET", "otpauth_url": "otpauth://totp/YourApp:label?..." }
```

### Endpoint: POST `/auth/mfa/totp/verify`

Body

```json
{ "code": "123456" }
```

Response

- 200 OK when the code matches; user remains logged in; subsequent requests are allowed.
- 400/401 on error.

### Built-in Setup Page (Browser)

- When the Accept header includes `text/html`, blocked requests are redirected to `/static/mfa/index.html`.
- This page:
  - Calls `/auth/mfa/totp/setup`, shows the secret and otpauth URL.
  - Posts the code to `/auth/mfa/totp/verify`.
  - Redirects to `/` on success.

## OIDC / Social Sign-In (Microsoft, Google, GitHub)

### Endpoint: GET `/auth/{provider}`

- Providers: `microsoft`, `google`, `github`.
- Redirects to provider authorization page.

### Endpoint: GET `/auth/{provider}/callback`

- Handles state/nonce verification, token exchange, and identity extraction.
- If the identity is already linked to a user: issues a session and selects an existing org for that user.
- If a user exists with the same email but no link yet: explicit linking is required (no auto-link). A short-lived pending token is set (cookie) and the client should navigate to a link-completion UI.
- If no user exists with that email: the server does NOT auto-create a user or auto-assign an org. It redirects to `/account/register?reason=signup_required&provider=...&email=...` (or returns 409 JSON).
- Configure `frontend.url` in server config to ensure safe redirect targets. Avoid relying on header-derived fallbacks in production.
- App does not enforce TOTP after OIDC sign-in (rely on IdP MFA).

### Identity Linking (existing email, new provider)

- Pending link is created on callback for existing emails without a link.
- Complete via `POST /auth/link/password` with local password (and TOTP if enabled). On success, identity is linked and session is issued.
## Current User and Authorization

### Endpoint: GET `/auth/me`

Response (example)

```json
{ "email": "user@example.com", "name": "User Name", "org": "acme", "role": "Admin", "provider": "google" }
```

Protected routes require a valid session; missing/invalid session yields 401.
Role-protected routes return 403 when role is insufficient.

## Request Patterns (Frontend)

- Always send requests with credentials (cookies): `fetch(url, { credentials: 'include' })`.
- Handle common errors:
  - 401 Unauthorized: redirect to login.
  - 403 with JSON `{ "error": "mfa_required" }`: redirect to `/static/mfa/index.html`.
  - 429 Too Many Requests: backoff and retry later.
- Include/propagate a `X-Request-ID` if you manage correlation; server also returns one.

## Example Flows

### Local Login with TOTP Required

1. POST `/auth/login` with username/password.
2. If you get 401 with message about MFA: retry with `totp_code`.
3. If MFA is globally required and user is not enrolled:
   - Navigations to app routes will redirect to `/static/mfa/index.html`.
   - Complete setup and verification.
   - Continue to the app.

### Microsoft/Google/GitHub Sign-In

1. Navigate to `/auth/{provider}`.
2. After consent, callback sets the session and redirects into the app.
3. Call `/auth/me` to hydrate the frontend session state.

## Admin Sessions (Troubleshooting)

### Endpoint: GET `/admin/sessions`

- Requires authenticated user with `Admin` or `Owner` role.
- Returns active sessions with `id`, `user_id`, `org_id`, `provider`, `expires_at`.
  - Security note: Treat `id` (session identifier) as sensitive bearer material. Do not log or display it outside trusted admin tooling. Future versions may omit or truncate this value.

## Security Notes

- Always set `frontend.url` in production so auth callbacks redirect to a known base URL; otherwise the server may infer a destination from request headers which is unsafe in untrusted edge setups.
- Ensure `security.session.cookie_secure=true` in production. If cross-site auth flows require `SameSite=none`, `Secure` must be true.
- For GitHub sign-in, prefer using the account's primary, verified email; if not available, prompt users to link to a local account rather than synthesizing an address.

## CORS and Static

- CORS allows local dev origins (see `cmd/server/main.go`).
- Static assets are served under `/static/*`.

## Notes

- Time must be accurate for TOTP; ensure client and server clocks are in sync.
- Denylist may block specific users or identities (provider+subject) with 403.
- Rate limit returns 429 with `Retry-After` set.
