Title: Org‑scoped EAV tables with schema, search, lookups, and admin endpoints

Summary
- Added a full set of org‑scoped “user tables” features on top of the existing auth stack: table/column/row management, search with schema, indexed lookups, row lookup by UUID, and delete operations. Also improved error handling and seeding.

Highlights
- Org scoping
  - `app.tables` now has `org_id` with per‑org uniqueness (`(org_id, slug)`), allowing different orgs to have different schemas for the same table slug.
  - Queries prefer the org‑specific table definition and allow a global fallback where appropriate.

- Row JSON fix
  - Hardened `app.row_to_json(uuid)` to never return NULL by coalescing each aggregate step.

- Search improvements
  - `POST /tables/{table}/search` now returns `{ columns, content, total_count }` with content unpacked from row JSON. If no filters are provided, search returns most recent rows by `created_at`.

- Table management
  - `GET /tables/` — list org tables
  - `POST /tables/` — create table (slug from name)
  - `DELETE /tables/{table}` — delete org table

- Column management
  - `POST /tables/{table}/columns` — add column (supports text/date/bool/enum/uuid/float; enum values; references; require_different_table; auto‑index via `ensure_index`)
  - `DELETE /tables/{table}/columns/{column}` — remove a column (cascades to values_* by FK)

- Row management
  - `POST /tables/{table}/rows` — insert row (type‑checked via PL/pgSQL)
  - `DELETE /tables/{table}/rows/{row_id}` — delete a row (org/table scoped)

- Indexed lookups and UUID exposure
  - `POST /tables/{table}/rows/indexed` — returns `{ id, label }` items using an indexed label column (prefers `title`, then indexed text/enum, then any text/enum)
  - `POST /tables/rows/lookup` — returns composed row JSON for a given UUID `{ id }`
  - `GET /tables/indexed-fields` — lists text/enum indexed fields per table for building cross‑table references

- Error mapping
  - Added `PGErrorMessage()` to map common Postgres errors to HTTP statuses + safe messages (unique violations, not‑null, check constraints, invalid format, raise exception messages from functions).

- Seed data
  - Expanded `017_seed_wos.up.sql` with multiple North Shore Wind sample work orders.

Files and changes (key)
- database/schema
  - 018_search_tables.up.sql — fixed `row_to_json` NULL propagation
  - 020_org_scoping.(up|down).sql — add `org_id` to `app.tables`, unique indexes per org
  - 017_seed_wos.up.sql — added realistic seed work orders

- database/queries/user_tables.sql — new/updated queries
  - SearchUserTable: org‑aware resolution; default to newest when no filters
  - GetUserTableSchema: returns table columns for rendering
  - CreateUserTable, ListUserTables, DeleteUserTable
  - AddUserTableColumn, RemoveUserTableColumn
  - InsertUserTableRow, DeleteUserTableRow
  - GetRowData (row JSON by UUID)
  - LookupIndexedRows (UUID+label), ListIndexedColumns (for cross‑table reference building)

- internal/db/gen — corresponding generated bindings added/updated

- internal/repo
  - Interface now includes table/column/row management, lookups, and helper methods
  - Implementations in `user_tables.go` for all new operations

- internal/handlers/tables
  - Endpoints added:
    - Create (POST /tables/)
    - List (GET /tables/)
    - Delete (DELETE /tables/{table})
    - AddColumn (POST /tables/{table}/columns)
    - RemoveColumn (DELETE /tables/{table}/columns/{column})
    - AddRow (POST /tables/{table}/rows)
    - DeleteRow (DELETE /tables/{table}/rows/{row_id})
    - Search (POST /tables/{table}/search) — now returns schema + unpacked rows + total_count
    - LookupIndexed (POST /tables/{table}/rows/indexed)
    - LookupRow (POST /tables/rows/lookup)

- internal/http
  - `pgerrors.go` — user‑friendly Postgres error mapping helper

Operational notes
- Index creation: ensure indexed columns have GIN/B‑tree created via `app.ensure_index`. You can trigger for existing columns with:
  - `SELECT app.ensure_index(id) FROM app.columns WHERE is_indexed;`
- Planner stats: `ANALYZE app.values_text; ANALYZE app.values_enum;` after large changes

Follow‑ups (optional)
- Add RLS policies using `current_setting('app.org_id')` and set per request/tx
- Add PATCH endpoints for updating rows/columns
- Add clone/copy schema utilities between orgs

