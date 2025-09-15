# EAV User Tables Audit (Org‑Scoped)

This document records a security and performance review of the org‑scoped EAV (Entity–Attribute–Value) feature and proposes concrete actions. Items are ordered by priority.

## Top Risks
- RLS missing: Access is org‑filtered in code, but the database does not enforce org isolation. A missed filter can leak data across orgs.
- Row lookup by UUID not org‑scoped: `POST /tables/rows/lookup` returns any row regardless of org.
- UUID reference integrity: `values_uuid.value` can reference any `app.rows` without enforcing target table/org.
- Write‑path fallback: Some write queries (e.g., insert row) resolve tables with `org_id IS NULL` fallback, potentially mixing multi‑org data into a global table.

## Recommended Changes

### 1) Add Row Level Security (RLS)
- Enable RLS on `app.tables`, `app.rows`, and all `app.values_*` tables.
- Policy: `USING (org_id = current_setting('app.org_id')::uuid)`.
- In app code, set per‑request/transaction: `SET LOCAL app.org_id = '<org uuid>'`.

### 2) Org‑scope the row lookup
- Update `GetRowData` to require `org_id` and join `app.rows -> app.tables` to verify the row belongs to the active org.

### 3) Add `org_id` to `app.rows` (Phase 2)
- Column: `org_id uuid NOT NULL`.
- Constraint: composite FK `(org_id, table_id) REFERENCES app.tables(org_id, id)`.
- Index: `app.rows(org_id, table_id, created_at DESC, id)`.
- Update `app.insert_row` / `app.update_row` to set/validate `org_id`.

### 4) Tighten write paths
- Remove `org_id IS NULL` fallback for INSERT/DDL (insert row, add/remove column, delete row/table). Reads can optionally retain fallback if explicitly desired.

### 5) Enforce reference integrity
- Add trigger(s) to ensure `values_uuid.value` references a row from the expected `columns.reference_table_id` (and same org if required).

### 6) Misc correctness & performance
- `app.row_to_json`: ensure no key collisions for `id`/`created_at` with user columns (reserve or nest under `_meta`).
- `app.update_row`: align casts with `insert_row` by extracting scalars via `#>> '{}'` before casting.
- Search COUNT(*) OVER(): consider separate count if needed for very large datasets.
- Ensure `app.ensure_index` ran for all indexed columns; run ANALYZE after bulk operations.

## Work Plan
- Phase A (now):
  - Org‑scope `GetRowData` (handler + repo + SQL).
  - Remove write‑path fallback in `InsertUserTableRow`.
- Phase B:
  - Add `org_id` to `app.rows` with constraints and indexes.
  - Update insert/update functions.
- Phase C:
  - Add RLS policies + session setting helper.
- Phase D:
  - UUID reference integrity triggers.
  - Optional: refine search filter validation and response metadata.

## Notes
- All endpoints already require auth; add role checks (Owner/Admin) for schema changes (create/delete table, add/remove columns).
- Document maintenance: force index creation for pre‑existing indexed columns and planner warmup (`ANALYZE`).
