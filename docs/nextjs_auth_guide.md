# Next.js Frontend Auth Guide (with Opaque Sessions + TOTP)

This guide shows how to wire a Next.js app to the Go API in this repo. It covers local auth (username/password + TOTP), OIDC sign-in (Microsoft/Google/GitHub), handling MFA, and two integration modes: direct CORS or same-origin proxy.

> Production Notes / Security Checklist
>
> - Configure the API with a `frontend.url` (and `frontend.post_login_path`) so OAuth/OIDC callbacks redirect to a known base URL. Do not rely on header-derived fallbacks.
> - Ensure cookies are secure: when running cross-site (different domains), set backend cookie to `SameSite=None` and `Secure=true` (HTTPS required). In same-site dev, `SameSite=Lax` is fine.
> - Always use `credentials: 'include'` in fetch calls so the `session` cookie is sent.
> - Prefer invitation-based org membership; avoid public self-join via `org_slug` in production.
> - Treat session IDs as secrets; do not log or display them in admin UIs.

## Integration Modes

- Direct CORS (simple)
  - Keep API at `http://127.0.0.1:8080` and Next.js at `http://localhost:3000`.
  - All fetches use `credentials: 'include'` to send/receive the `session` cookie.
  - Ensure your API `CORS` allows the Next.js origin (already set in server).

- Same-origin reverse proxy (recommended)
  - Use a Next.js rewrite so your app talks to `/api/*` and the dev server proxies to the Go API.
  - Cookies become first-party to your Next.js domain, enabling SSR if needed.

### next.config.js (proxy option)

```js
/** @type {import('next').NextConfig} */
const nextConfig = {
  async rewrites() {
    return [
      {
        source: '/api/:path*',
        destination: 'http://127.0.0.1:8080/:path*', // Go API
      },
      {
        source: '/static/:path*',
        destination: 'http://127.0.0.1:8080/static/:path*', // serve MFA page, etc.
      },
    ]
  },
}
module.exports = nextConfig
```

Then use relative paths like `/api/auth/login`. Otherwise, use an `API_BASE` env and direct URLs. Also set the server's `frontend.url` to the app origin to ensure safe redirects after OIDC.

## Environment

```env
# If using direct CORS:
NEXT_PUBLIC_API_BASE_URL=http://127.0.0.1:8080
```

## API Helper (fetch wrapper)

```ts
// lib/api.ts
export const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL || '' // '' when using proxy

export async function apiFetch(input: string, init: RequestInit = {}) {
  const url = API_BASE ? API_BASE + input : input
  const res = await fetch(url, {
    ...init,
    credentials: 'include',
    headers: {
      'X-Requested-With': 'fetch',
      ...(init.headers || {}),
    },
  })

  // Handle MFA-required for API clients
  if (res.status === 403) {
    try {
      const data = await res.clone().json()
      if (data?.error === 'mfa_required') {
        // Browser redirect to MFA setup page exposed by the API
        if (typeof window !== 'undefined') {
          const target = API_BASE ? `${API_BASE}/static/mfa/index.html` : '/static/mfa/index.html'
          window.location.href = target
        }
      }
    } catch {}
  }

  return res
}
```

## Auth State (client-side)

Use SWR or React Query to hydrate the current user from `/auth/me`.

```tsx
// app/auth-provider.tsx
'use client'
import useSWR from 'swr'
import { apiFetch } from '@/lib/api'

export function useAuth() {
  const { data, error, isLoading, mutate } = useSWR('/auth/me', async (url) => {
    const res = await apiFetch(url)
    if (res.status === 401) return null
    if (!res.ok) throw new Error(await res.text())
    return res.json()
  })
  return { user: data, error, isLoading, refresh: mutate }
}

export function AuthGate({ children }: { children: React.ReactNode }) {
  const { user, isLoading } = useAuth()
  if (isLoading) return null
  if (!user) {
    if (typeof window !== 'undefined') window.location.href = '/login'
    return null
  }
  return <>{children}</>
}
```

Wrap protected pages/components with `AuthGate` or check `useAuth().user`.

## Login Page (local auth)

```tsx
// app/login/page.tsx
'use client'
import { useState } from 'react'
import { apiFetch } from '@/lib/api'

export default function LoginPage() {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [totp, setTotp] = useState('')
  const [msg, setMsg] = useState('')

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault()
    setMsg('')
  const res = await apiFetch('/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password, totp_code: totp || undefined }),
    })
    if (res.status === 200 || res.status === 201) {
      window.location.href = '/'
      return
    }
    if (res.status === 401) {
      setMsg('Invalid credentials or missing TOTP code')
      return
    }
    if (res.status === 403) {
      try {
        const data = await res.clone().json()
        if (data?.error === 'mfa_required') {
          window.location.href = '/static/mfa/index.html'
          return
        }
      } catch {}
    }
    setMsg('Login error: ' + (await res.text()))
  }

  function startOIDC(provider: 'microsoft' | 'google' | 'github') {
    const base = process.env.NEXT_PUBLIC_API_BASE_URL || ''
    window.location.href = `${base}/auth/${provider}`
  }

  return (
    <div style={{ maxWidth: 420, margin: '3rem auto' }}>
      <h1>Sign in</h1>
      <form onSubmit={onSubmit}>
        <div>
          <input value={username} onChange={e => setUsername(e.target.value)} placeholder="username or email" />
        </div>
        <div>
          <input type="password" value={password} onChange={e => setPassword(e.target.value)} placeholder="password" />
        </div>
        <div>
          <input value={totp} onChange={e => setTotp(e.target.value)} placeholder="TOTP code (if enabled)" />
        </div>
        <button type="submit">Sign in</button>
      </form>

      <hr />
      <button onClick={() => startOIDC('microsoft')}>Sign in with Microsoft</button>
      <button onClick={() => startOIDC('google')}>Sign in with Google</button>
      <button onClick={() => startOIDC('github')}>Sign in with GitHub</button>
      <div style={{ color: 'crimson' }}>{msg}</div>
    </div>
  )
}
```

## Protecting Routes

- Client-side: wrap pages with `AuthGate` or check `useAuth().user`.
- SSR (proxy mode): you can fetch `/api/auth/me` from server components and read cookies automatically. Example:

```tsx
// app/(protected)/layout.tsx (App Router, proxy mode only)
import { cookies } from 'next/headers'

async function getUser() {
  // Next forwards cookies to the proxied backend when using rewrites.
  const res = await fetch('http://localhost:3000/api/auth/me', { cache: 'no-store' })
  if (res.status === 401) return null
  if (!res.ok) throw new Error('failed to fetch user')
  return res.json()
}

export default async function ProtectedLayout({ children }: { children: React.ReactNode }) {
  const user = await getUser()
  if (!user) {
    return <meta httpEquiv="refresh" content="0;url=/login" />
  }
  return <>{children}</>
}
```

If you’re not using the proxy, SSR can’t see the API cookie (different domain). Prefer client-side protection.

## MFA Setup Page

The backend serves a simple page at `/static/mfa/index.html` that:
- Calls `/auth/mfa/totp/setup` to get the secret + otpauth URL.
- Posts `/auth/mfa/totp/verify` with the 6-digit code.
- Redirects to `/` on success.

You can replace it with a Next.js page that makes the same calls via `apiFetch`.

## Logout

```ts
// Simple client-side logout
await apiFetch('/auth/logout', { method: 'POST' })
window.location.href = '/login'
```

## Error Handling

- 401 Unauthorized → redirect to `/login`.
- 403 `{ "error": "mfa_required" }` → redirect to `/static/mfa/index.html` (or your Next.js MFA page).
- 429 Too Many Requests → show a friendly backoff message.
- Network errors → show a retry option.

## Tips

- Always call `fetch` with `credentials: 'include'`.
- For OIDC, redirect the browser to `${API_BASE}/auth/{provider}`; the server will set the cookie and redirect back into the app.
- Use the proxy mode for SSR and nicer cookie behavior in development.
