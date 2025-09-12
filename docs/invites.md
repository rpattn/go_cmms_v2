# Organisation Invites

This document outlines the upcoming invite flow that complements the new signup model (creator becomes Owner for a new organisation).

## Goals
- Existing Owners can invite users to their organisation.
- Invited users accept via a short-lived token and are granted a role (default Member).
- Support email delivery, token expiry, and revocation.

## Endpoints

- POST `/auth/invite` (Owner required)
  - Body: `{ "email": "user@example.com", "role": "Member" }`
  - Creates an invite and returns an `accept_url` plus `exp` and `role`.
  - The server stores only a SHA-256 hash of the underlying token; the token is embedded in the URL query string.
  - The `accept_url` is constructed from `frontend.url` + `frontend.api_route` + `/invite/accept?token=...`.
  - Returns: `{ "ok": true, "accept_url": "https://api.example.com/invite/accept?token=...", "exp": "2025-09-14T...Z", "role": "Member" }`

- POST `/auth/invite/accept`
  - Body: `{ "token": "..." }`
  - Requires a logged-in user. The user’s email must match the invite email.
  - Adds membership with the invited role and marks the invite used.
  - Returns: `{ "ok": true }` and switches the session’s `active_org` to the invited org.

## Schema (proposed)

```sql
CREATE TABLE org_invites (
  token TEXT PRIMARY KEY,
  org_id UUID NOT NULL REFERENCES organisations(id) ON DELETE CASCADE,
  email TEXT NOT NULL,
  role org_role NOT NULL DEFAULT 'Member',
  inviter_id UUID NOT NULL REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at TIMESTAMPTZ NOT NULL,
  used_at TIMESTAMPTZ
);
CREATE INDEX org_invites_email_idx ON org_invites (email);
```

## Flow
1. Owner creates invite; server returns `accept_url` and expiry. Deliver the URL to the recipient (e.g., email).
2. Recipient logs in or signs up (email must match the invitation).
3. Frontend posts token to `/auth/invite/accept`.
4. Backend validates token (hash match), checks expiry/usage, verifies email match, adds membership, marks invite used, and switches the active org in the session.

## Security Notes
- Store a hash of the token; never persist plaintext tokens.
- Expire tokens (e.g., 7 days). Support revocation.
- Rate-limit invite creation and acceptance endpoints.
- Email verification: require the accepting user’s email to match the invited email or enforce extra verification.

Status: implemented (token generation, storage, accept). Email delivery is application-specific and not included.

### Config
- `frontend.url`: the public origin of your frontend (e.g., `https://app.example.com`).
- `frontend.api_route`: the path prefix on the frontend that proxies to the backend (default empty). If set (e.g., `/api/backend`), it is used when building acceptance links so they route through your frontend.

## Static Pages

For quick manual testing and simple admin flows, two static pages are provided and served by the backend under `/static/`:

- Create: `/static/invite/index.html` — Owners can create invites and copy the token or a ready-made accept link.
- Accept: `/static/invite/accept.html?token=...` — Logged-in recipients can paste the token to accept and join the org.

These pages use `fetch` with `credentials: 'include'` to send the session cookie.
