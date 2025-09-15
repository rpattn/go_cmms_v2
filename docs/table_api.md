# Table API Endpoints

All endpoints require authentication and use the active organisation from the session context. Paths are relative to your API root.

- Auth: Bearer session cookie/token established via your auth flow
- Org scope: Endpoints operate on tables owned by the current org

Tables
- GET `/tables/`: List org tables
  - Response: `{ "tables": [{ id, name, slug, created_at }, ...] }`
- POST `/tables/`: Create a table
  - Body: `{ "name": "Work Orders" }`
  - Response: `201 { "created": true|false, "table": { id, name, slug, created_at } }`
- DELETE `/tables/{table}`: Delete a table (by slug or name)
  - Response: `{ "deleted": true, "table": { id, name, slug, created_at } }`
- GET `/tables/indexed-fields`: List indexed text/enum fields (for cross‑table references)
  - Response: `{ "items": [{ table_id, table_slug, table_name, column_id, column_name, column_type }, ...] }`

Columns
- POST `/tables/{table}/columns`: Add a column
  - Body:
    - `{ "name": "title", "type": "text", "required": true, "indexed": true }`
    - Enum: `{ "name": "priority", "type": "enum", "enum_values": ["LOW","MEDIUM","HIGH"], "indexed": true }`
    - Reference: `{ "name": "customer", "type": "uuid", "is_reference": true, "reference_table": "customers", "require_different_table": true }`
  - Response: `201/200 { "created": true|false, "column": { id, name, type, required, indexed, enum_values?, is_reference, reference_table_id?, require_different_table } }`
- DELETE `/tables/{table}/columns/{column}`: Remove a column
  - Response: `{ "deleted": true, "column": { ...deleted column details... } }`

Rows
- POST `/tables/{table}/rows`: Insert a row
  - Body: JSON object with column values, e.g. `{ "title":"Replace filter","priority":"MEDIUM","required_signature":false }`
  - Response: `201 { "row": { "row_id": "<uuid>", "data": { ... }, "total_count": 0 } }`
- DELETE `/tables/{table}/rows/{row_id}`: Delete a row by UUID
  - Response: `{ "deleted": true, "row_id": "<uuid>" }`

Search
- POST `/tables/{table}/search`: Search rows with schema
  - Body: `{ "pageNum": 0, "pageSize": 10, "filterFields": [{ "field":"status", "operation":"eq", "value":"OPEN" }] }`
    - Supported operations by type:
      - text: `eq`, `cn` (contains), `in` (array of values)
      - enum: `eq`, `in`
      - bool: equality (true/false)
  - Response: `{ "columns": [{ id,name,type,required,indexed,enum_values?,... }], "content": [ { ...row data... }, ... ], "total_count": N }`
  - Notes: If `filterFields` is missing/empty, returns most recent rows by `created_at`.

UUID Lookups
- POST `/tables/{table}/rows/indexed`: Minimal list for UI selectors
  - Body: `{ "field":"title", "q":"fil", "limit":20 }` (all optional)
  - Picks label column by preference: `title` → indexed text/enum → any text/enum
  - Response: `{ "items": [{ "id":"<uuid>", "label":"..." }, ...] }`
- POST `/tables/rows/lookup`: Get composed JSON for a UUID
  - Body: `{ "id":"<uuid>" }`
  - Response: `{ "data": { ...row json... } }`

Examples
- Create table
  - `curl -X POST http://localhost:8080/tables/ -H "Authorization: Bearer TOKEN" -H "Content-Type: application/json" -d '{"name":"Customers"}'`
- Add column
  - `curl -X POST http://localhost:8080/tables/customers/columns -H "Authorization: Bearer TOKEN" -H "Content-Type: application/json" -d '{"name":"name","type":"text","required":true,"indexed":true}'`
- Insert row
  - `curl -X POST http://localhost:8080/tables/customers/rows -H "Authorization: Bearer TOKEN" -H "Content-Type: application/json" -d '{"name":"Acme Inc."}'`
- Indexed lookup
  - `curl -X POST http://localhost:8080/tables/customers/rows/indexed -H "Authorization: Bearer TOKEN" -H "Content-Type: application/json" -d '{"q":"ac"}'`
- Search
  - `curl -X POST http://localhost:8080/tables/customers/search -H "Authorization: Bearer TOKEN" -H "Content-Type: application/json" -d '{"pageNum":0,"pageSize":10,"filterFields":[{"field":"name","operation":"cn","value":"ac"}]}'`
- Delete row
  - `curl -X DELETE http://localhost:8080/tables/customers/rows/<uuid> -H "Authorization: Bearer TOKEN"`
- Delete table
  - `curl -X DELETE http://localhost:8080/tables/customers -H "Authorization: Bearer TOKEN"`

Notes
- All endpoints return user‑friendly error messages with appropriate HTTP statuses (unique constraint → 409, invalid format → 400, etc.).
- Table/column names are case‑insensitive in API routes; slugs are lowercase by design.

