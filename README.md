# Go Server

A starting **Go Server** built with:

- Multi-tenant architecture (users, organizations, roles).
- Authentication via **username/password**, **TOTP MFA**, and **OAuth (Google, Microsoft)**.
- Role-based access control (Owner, Admin, Member, Viewer).
- PostgreSQL-backed persistence, generated queries with **sqlc**.
- Database migrations with **golang-migrate**.
- Configurable via `config.yaml` or environment variables.
- HTTP server built on **chi**, secure session cookies, and static asset serving.
- Modular design for scaling into a full CMMS with transaction logging.

---

## 🚀 Project Goals

- Provide a robust authentication/authorization foundation.
- Support organizations, roles, and multi-tenant security.

---

## 🏗 Project Architecture

<pre>
go_server/
│
├── cmd/
│ └── server/ # Entrypoint for the HTTP server (main.go)
│
├── internal/
│ ├── auth/ # Auth flows: signup, login, logout, MFA (TOTP), OAuth
│ ├── config/ # Config loader (env + YAML)
│ ├── middleware/ # Session and logging middleware
│ ├── models/ # Domain models (User, Org, Role, Credential, etc.)
│ ├── providers/ # OAuth providers (Google, Microsoft)
│ └── repo/ # Repository implementation (wraps sqlc generated code)
│
├── database/
│ ├── migrations/ # SQL migrations (with golang-migrate)
│ ├── queries/ # Handwritten SQL queries for sqlc
│ └── gen/ # sqlc-generated Go code
│
├── static/ # Static assets (test.html)
│
├── scripts/ # PowerShell scripts for install, migrate, sqlc, run
│
├── examle.config.yaml # Default config (example)
└── go.mod # Go module definition
</pre>

---

## 🔑 Authentication Features

- **Local auth**
  - Signup with email, username, password.
  - Passwords hashed with **Argon2id** in PHC string format.
  - TOTP MFA setup/verification (Google Authenticator-compatible).
- **OAuth**
  - Microsoft and Google login support.
  - Automatic user creation via verified email.
  - Organization role mapping from IdP groups.
- **Sessions**
  - Secure session cookies (`HttpOnly`, `Secure`, `SameSite`).

---

## ⚙️ Setup & Development

### Prerequisites
- Go 1.22+
- PostgreSQL 14+
- [sqlc](https://sqlc.dev) (query codegen)
- [golang-migrate](https://github.com/golang-migrate/migrate) (migrations)
- On Windows: use [scoop](https://scoop.sh) for easy installation.

### Install tools
```powershell
scoop install sqlc migrate
```

### Run migrations
`.\scripts\migrate-up.ps1`

### Down migrations
`.\scripts\migrate-down.ps1`

### Generate sql code
`.\scripts\sqlc-generate.ps1`

### Run server
`.\scripts\migrate-up.ps1`

Visit http://localhost:8080/static/test.html
 to use the test UI.


## Configure

Edit config.yaml (or use env vars):

base_url: "http://localhost:8080"

database:
  url: "postgres://postgres:postgres@localhost:5432/cmms?sslmode=disable"

oauth:
  google:
    client_id: "xxx"
    client_secret: "xxx"
  microsoft:
    client_id: "xxx"
    client_secret: "xxx"


## Test

Testing the API

POST /auth/signup → signup new user.

POST /auth/login → login with username/password (+ TOTP if enabled).

POST /auth/logout → clear session cookie.

GET /auth/mfa/totp/setup → provision TOTP secret + QR.

POST /auth/mfa/totp/verify → validate TOTP setup.

A sample test.html is served at /static/test.html with forms for signup/login and logs to browser console.

Users can sign up without providing an org, and they are assigned to the test acme org (see see_demo.(up/down).sql to change this)

Users can also sign up without an org and we try to map from their email domain (e.g. @testorg.com --> testorg slug)

## Why Go?

Performance: Compiles to a single static binary, fast concurrency with goroutines.

Safety: Strong typing, no hidden magic.

Ecosystem: sqlc, chi, pgx, pquerna/otp → strong libraries with minimal runtime overhead.

Deployment: Easy to ship anywhere (Docker, bare metal, cloud).

Scalability: Well-suited for multi-tenant SaaS and high-concurrency APIs.

---

## App Overview (User Tables + Org Scoping)

This server includes a dynamic, org-scoped EAV model for user-defined tables:

- Define tables and columns at runtime per organisation
- Store rows with strongly typed values (text/date/bool/enum/uuid/float)
- Search with filters and pagination, returning row JSON plus table schema
- Manage columns (add/remove) and rows (insert/delete)
- Indexed lookups expose UUIDs + human labels for cross-table references
- Friendly Postgres error messages mapped to clean API responses

### Key Endpoints (selected)

- Tables
  - GET `/tables/` — list tables
  - POST `/tables/` — create a table `{ name }`
  - DELETE `/tables/{table}` — delete a table
  - GET `/tables/indexed-fields` — list indexed text/enum fields per table

- Columns
  - POST `/tables/{table}/columns` — add a column
  - DELETE `/tables/{table}/columns/{column}` — remove a column

- Rows
  - POST `/tables/{table}/rows` — insert a row
  - DELETE `/tables/{table}/rows/{row_id}` — delete a row
  - POST `/tables/{table}/search` — search; response `{ columns, content, total_count }`
  - POST `/tables/{table}/rows/indexed` — list `{ id, label }` for lookups
  - POST `/tables/rows/lookup` — get composed JSON by UUID `{ id }`

All routes are org-scoped via the authenticated session.


## TODO
Fix filtering
uuid array support