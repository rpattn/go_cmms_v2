# Table-per-Collection Migration Plan

## 1. Current EAV Architecture Snapshot
- Logical tables, columns, rows, and value buckets are stored in the `app` schema (`app.tables`, `app.columns`, `app.rows`, `app.values_*`). Columns capture type, required/indexed flags, enum metadata, and reference hints for UUID fields.【F:database/schema/006_user_tables.up.sql†L1-L83】
- `app.tables` is already org-scoped via an `org_id` foreign key and per-org uniqueness on slugs/names.【F:database/schema/020_org_scoping.up.sql†L1-L19】
- Inserts and updates pivot JSON payloads through `app.insert_row` / `app.update_row`, routing values into the per-type buckets and checking required columns.【F:database/schema/012_insert_rows.up.sql†L1-L98】【F:database/schema/013_update_rows.up.sql†L1-L49】
- Reads assemble row JSON via `app.rows_json`, while `SearchUserTable` filters per type and joins bucket tables dynamically. sqlc-generated DAO methods wrap these SQL entrypoints for the HTTP layer.【F:database/schema/014_read_rows.up.sql†L1-L38】【F:database/queries/user_tables.sql†L1-L200】【F:internal/repo/user_tables.go†L48-L141】
- UUID references are guarded only by trigger-time checks in `app.enforce_uuid_reference`, not true foreign keys, and indexes are synthesized per column by `app.ensure_index` over the EAV buckets.【F:database/schema/008_cross_table_refs.up.sql†L1-L38】【F:database/schema/009_indexable.up.sql†L1-L43】
- The REST API expects schema introspection, CRUD, search, and lookup endpoints to keep their current payloads and semantics.【F:docs/table_api.md†L1-L74】

## 2. Target Table-per-Collection Design
### 2.1 Physical tables
- Every user collection maps 1:1 to a dedicated physical table in a dedicated application schema (e.g. `app_data`). Table name can be derived from the org/table metadata, e.g. `app_data.t_{org_short_id}_{table_id}` to avoid collisions and keep names short.
- Physical table columns:
  - `id uuid PRIMARY KEY DEFAULT gen_random_uuid()` (retains existing row identifiers).
  - `org_id uuid NOT NULL` with FK to `organisations(id)` for belt-and-suspenders multi-tenancy. Optionally store `table_id bigint NOT NULL` referencing `app.tables(id)` with a `CHECK (table_id = <metadata table id>)` enforced via triggers to protect against rename churn.
  - `created_at timestamptz NOT NULL DEFAULT now()` plus optional `updated_at` maintained via trigger.
  - One column per logical column, using sanitized physical names from metadata and native Postgres types.

### 2.2 Metadata changes
- Keep `app.tables` / `app.columns` but add physical metadata fields:
  - `app.tables.physical_table_name text NOT NULL` (server-side generated), plus optional `schema_name` (defaults to `app_data`).
  - `app.columns.physical_column_name text NOT NULL` so runtime DDL knows the exact identifier. Maintain a generated slug (e.g. `c_<id>` or sanitized name with suffix) to avoid renaming collisions.
  - Store enum constraint backing type names (e.g. `enum_type_name text`) and index names if needed for reversibility.
- Add a migration status flag on `app.tables` (`storage_mode enum('eav','relational')`, `migrated_at timestamptz`) to orchestrate dual-write and cut-over.

### 2.3 Column typing & constraints
- Map logical types to native SQL types:
  - `text` → `text` with optional `CHECK (char_length(col) > 0)` if required.
  - `date` → `date`.
  - `bool` → `boolean`.
  - `float` → `double precision` or `numeric`.
  - `uuid` → `uuid`.
  - `enum` → dedicated Postgres enum type per column (e.g. `app_data.e_{column_id}`) or a table-scoped `CHECK (col = ANY(<value list>))`. Enum types should be owned by the column for safe drop/alter semantics.
- When `is_required` is true, enforce `NOT NULL` at the column level.
- When `is_indexed` is true, create an index that matches the old semantics:
  - `GIN (col gin_trgm_ops)` for text.
  - `btree` for date/float/bool/uuid/enum.
- For `is_reference` UUID columns, emit `ALTER TABLE ... ADD CONSTRAINT ... FOREIGN KEY (col) REFERENCES <target_table>(id) ON DELETE RESTRICT`. `require_different_table` becomes a `CHECK (col IS NULL OR col <> id)` or multi-column constraint referencing the table id, enforced with triggers if cross-table knowledge is needed.

### 2.4 DDL helpers
- Replace `app.insert_row` / `app.update_row` with helper functions that operate on the per-table relations or build the SQL from Go using prepared statements. Provide a single function `app.ensure_physical_table(p_table_id bigint)` that creates/updates the physical table using metadata (transactionally) and records completion.
- Introduce `app.add_physical_column(p_column_id bigint)` that runs `ALTER TABLE ... ADD COLUMN`, `ALTER TYPE/ADD VALUE` for enums, attaches FKs, and creates indexes.
- Provide `app.drop_physical_column(p_column_id bigint)` and `app.drop_physical_table(p_table_id bigint)` for cleanup, ensuring dependent constraints/types are removed safely.

## 3. Runtime Workflow Changes
### 3.1 Create table
1. Insert metadata row as today (`CreateUserTable`).【F:database/queries/user_tables.sql†L114-L135】
2. In the same transaction, call `app.ensure_physical_table` to materialize the table with baseline columns (`id`, `org_id`, `table_id`, timestamps) and mark the table as relational. Failure should roll back metadata creation.
3. Return metadata to API unchanged.

### 3.2 Add column
1. Keep metadata insert (so API semantics stay idempotent).【F:database/queries/user_tables.sql†L137-L199】
2. After commit, run `app.add_physical_column` which:
   - Computes physical column name/type.
   - Issues `ALTER TABLE <physical> ADD COLUMN ...` with `NOT NULL DEFAULT` if required (followed by `ALTER COLUMN DROP DEFAULT`).
   - Applies enum constraint (create enum type or check) and `ALTER TYPE ... ADD VALUE` for later enum extensions.
   - Creates indexes per `is_indexed` via deterministic naming.
   - Adds FKs for references (ensuring referenced table already migrated).
3. Update metadata status if the table is still dual-writing.

### 3.3 Remove column
- Drop metadata then execute `ALTER TABLE ... DROP COLUMN` (with `CASCADE` for associated indexes/constraints) and drop the supporting enum type if unused.

### 3.4 Insert/update rows
- Build SQL dynamically in repository layer because table and column identifiers vary per org table. Example insert skeleton: `INSERT INTO <physical_table> (<columns...>) VALUES (...) RETURNING id, row_to_json(t.*)`.
- Use metadata to map API payload keys to column names/types. For updates, generate `UPDATE ... SET col = $n` statements on provided fields.
- Reuse current validation logic by moving it to Go: check `is_required`, enum membership, reference existence (via FK errors), etc. Additional server-side constraints provide double protection.
- Maintain label resolution by querying native columns instead of EAV buckets; update `BatchGetRowLabels` to use `SELECT id, <label_column> FROM <physical_table>`.

### 3.5 Search/read APIs
- Replace `SearchUserTable` SQL with dynamic query builders or SQL functions that accept table id and filter JSON, generating `WHERE` clauses per column type similar to the current case expression.【F:database/queries/user_tables.sql†L1-L83】
- Use `row_to_json` or `jsonb_build_object` on the physical rows to match the old `{ "data": { ... } }` shape expected by `internal/repo` and the HTTP handlers.【F:internal/repo/user_tables.go†L48-L78】
- Keep `GetUserTableSchema` metadata query unchanged aside from new physical columns; the handler continues to provide schema then rows to the client.【F:internal/repo/user_tables.go†L80-L117】【F:internal/handlers/tables/tables.go†L31-L95】

### 3.6 UUID lookups and references
- `LookupIndexedRows`, `BatchGetRowLabels`, and label resolution in the handler should pivot to the physical table/column rather than the EAV buckets, still returning `{id,label}` pairs.【F:internal/handlers/tables/tables.go†L59-L135】
- Foreign keys supply native integrity; the existing `app.enforce_uuid_reference` trigger can be retired once all UUID reference columns have FK constraints.【F:database/schema/008_cross_table_refs.up.sql†L1-L38】

## 4. Migration Strategy
1. **Schema prep migration**
   - Add new metadata columns/flags and helper enums to `app.tables`/`app.columns`.
   - Deploy stored procedures for ensuring physical tables/columns.
   - Leave existing EAV queries untouched (storage_mode=`eav`).
2. **Background backfill**
   - For each table, create its physical counterpart and columns based on metadata. Populate using batched `INSERT INTO physical (...) SELECT ... FROM app.rows + values_*` to minimize locks.
   - Validate row counts and sample data.
3. **Dual write window**
   - Update `app.insert_row`/`app.update_row` (or repository write path) to write to both EAV and physical tables when `storage_mode='dual'`. Keep reads on EAV until parity is confirmed.
   - For heavy write traffic, consider capturing changes via triggers or application-level fan-out.
4. **Cutover per table**
   - Toggle metadata to mark table as `relational` once diff-free.
   - Switch read queries to the physical tables for that table id; remove dual writes.
5. **Decommission EAV**
   - After all tables are migrated, drop EAV buckets and helper triggers/functions. Clean metadata columns that only applied to EAV (e.g., `enum_values` arrays) if redundant.

## 5. Application & sqlc Updates
- sqlc cannot parameterize table names, so new read/write paths will either:
  - Call plpgsql functions (`app.search_relational(p_table_id, p_payload)`) that internally build dynamic SQL, keeping sqlc-friendly signatures.
  - Or bypass sqlc for dynamic queries using pgx directly in `internal/repo/user_tables.go` for operations that require runtime identifiers.
- Update repository methods to leverage new helper functions while preserving method signatures so HTTP handlers stay untouched.【F:internal/repo/user_tables.go†L48-L141】
- Adjust models only if additional metadata needs to be surfaced; existing API contracts remain identical.【F:internal/models/types.go†L92-L141】【F:docs/table_api.md†L8-L74】

## 6. Testing & Deployment Considerations
- Unit tests: add coverage for metadata-to-DDL translation (sanitizing identifiers, enum creation, FK enforcement) and repository dynamic SQL builders.
- Integration tests: run through the REST flows (create table, add columns, insert/search/delete) against a Postgres instance to ensure parity with EAV responses.
- Migration dry runs: execute backfill on staging data, verifying row counts, constraints, and query plans. Monitor DDL locks and plan batches during low-traffic windows.
- Observability: add metrics/logging around `ensure_physical_table` / `add_physical_column` latency and FK violations to catch issues early.

## 7. Risks & Mitigations
- **DDL locking**: Creating or altering large tables can lock writes. Mitigate via `ALTER TABLE ... ADD COLUMN` with `DEFAULT NULL` (no rewrite) and scheduling heavy operations during maintenance windows.
- **Enum evolution**: Changing enum value sets requires `ALTER TYPE ... ADD VALUE`, which is monotonic. Plan UI flows accordingly or model enums as lookup tables to allow deletes.
- **Reference ordering**: When adding a UUID reference, ensure the target table is already migrated and referenced rows exist; rely on deferred FKs or transactional inserts to avoid deadlocks.
- **Rollback**: Retain EAV data until confident in relational storage; ability to reset `storage_mode` provides a safety hatch.

This plan keeps the external API unchanged while moving persistence to first-class relational tables, unlocking native constraints and better write amplification characteristics.
