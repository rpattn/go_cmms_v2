Title: Dual-write EAV tables with physical storage, search, and float support

## Summary
- Added dual-write infrastructure that keeps the canonical EAV buckets and per-table physical storage in sync (`app.ensure_physical_table`, `app.add_physical_column`, `app.insert_row_physical`, `app.delete_row_physical`).
- Extended the schema metadata and migrations to provision physical tables, create per-column enum/FK/index constraints, and track logical row timestamps for change detection.
- Fixed missing float coverage across insert/update/row JSON helpers so floats behave like the other scalar types end-to-end.
- Introduced physical-table search/read helpers while keeping existing endpoints org-scoped and backward compatible.
- Added PATCH endpoint to edit rows 

## Highlights
- `database/schema/021_physical_storage.up.sql` scaffolds metadata, physical DDL helpers, and enum/index/FK wiring for table-per-collection mode.
- `database/schema/022_dual_write.up.sql`, `028_insert_physical_ensure_cols.up.sql`, and `029_cutover_helpers.up.sql` implement insert/delete/backfill helpers that hydrate physical tables from canonical JSON.
- `database/schema/025_physical_search.up.sql` adds `app.search_user_table_physical` with filter/sort parity plus automatic fallback when a table has not been cut over.
- `database/schema/026_row_timestamps.up.sql` adds `updated_at`, ensures physical triggers, and rebuilds `app.update_row` to dual-write using canonical EAV JSON.
- `database/schema/027_rows_json_float_fix.up.sql` rewrites `app.rows_json` to include floats so search/read payloads stay accurate.
- `database/queries/user_tables.sql` and regenerated `internal/db/gen` wire the new helpers into the API surface, while `internal/repo/user_tables.go` adds a physical-search fast path when a raw connection is available.

## Testing
- Not run (pending CI)