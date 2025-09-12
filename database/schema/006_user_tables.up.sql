-- User-defined logical tables
CREATE TABLE IF NOT EXISTS app.tables (
  id           bigserial PRIMARY KEY,
  name         text NOT NULL,
  slug         text NOT NULL UNIQUE,
  created_at   timestamptz NOT NULL DEFAULT now()
);

-- Column types allowed
CREATE TYPE app.column_type AS ENUM ('text','date','bool','enum','uuid','float');

-- User-defined columns
CREATE TABLE IF NOT EXISTS app.columns (
  id                  bigserial PRIMARY KEY,
  table_id            bigint NOT NULL REFERENCES app.tables(id) ON DELETE CASCADE,
  name                text   NOT NULL,               -- API name (unique per table)
  type                app.column_type NOT NULL,
  is_required         boolean NOT NULL DEFAULT false,
  is_indexed          boolean NOT NULL DEFAULT false, -- if true, we'll ensure an index
  -- ENUM-only: allowed values
  enum_values         text[] DEFAULT NULL,
  -- UUID-only: treat as foreign reference to rows in another table?
  is_reference        boolean NOT NULL DEFAULT false,
  reference_table_id  bigint REFERENCES app.tables(id) ON DELETE RESTRICT,
  -- If true, require referenced row to come from a DIFFERENT table than the row being filled
  require_different_table boolean NOT NULL DEFAULT false,

  CONSTRAINT columns_table_name_unique UNIQUE (table_id, name),
  CONSTRAINT enum_values_only_for_enum CHECK ((type = 'enum') = (enum_values IS NOT NULL)),
  CONSTRAINT reference_only_for_uuid CHECK ((type = 'uuid') OR (NOT is_reference))
);

-- Every logical row has a stable global UUID
CREATE TABLE IF NOT EXISTS app.rows (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  table_id   bigint NOT NULL REFERENCES app.tables(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now()
);

-- Values are stored per type for strong typing + indexing flexibility
CREATE TABLE IF NOT EXISTS app.values_text (
  row_id    uuid    NOT NULL REFERENCES app.rows(id) ON DELETE CASCADE,
  column_id bigint  NOT NULL REFERENCES app.columns(id) ON DELETE CASCADE,
  value     text    NULL,
  PRIMARY KEY (row_id, column_id)
);

-- Values are stored per type for strong typing + indexing flexibility
CREATE TABLE IF NOT EXISTS app.values_float (
  row_id    uuid    NOT NULL REFERENCES app.rows(id) ON DELETE CASCADE,
  column_id bigint  NOT NULL REFERENCES app.columns(id) ON DELETE CASCADE,
  value     float    NULL,
  PRIMARY KEY (row_id, column_id)
);

CREATE TABLE IF NOT EXISTS app.values_date (
  row_id    uuid   NOT NULL REFERENCES app.rows(id) ON DELETE CASCADE,
  column_id bigint NOT NULL REFERENCES app.columns(id) ON DELETE CASCADE,
  value     date   NULL,
  PRIMARY KEY (row_id, column_id)
);

CREATE TABLE IF NOT EXISTS app.values_bool (
  row_id    uuid   NOT NULL REFERENCES app.rows(id) ON DELETE CASCADE,
  column_id bigint NOT NULL REFERENCES app.columns(id) ON DELETE CASCADE,
  value     boolean NULL,
  PRIMARY KEY (row_id, column_id)
);

CREATE TABLE IF NOT EXISTS app.values_enum (
  row_id    uuid   NOT NULL REFERENCES app.rows(id) ON DELETE CASCADE,
  column_id bigint NOT NULL REFERENCES app.columns(id) ON DELETE CASCADE,
  value     text   NULL,
  PRIMARY KEY (row_id, column_id)
);

-- UUID "value" columns can reference ANY row (optionally constrained to a specific target table)
CREATE TABLE IF NOT EXISTS app.values_uuid (
  row_id    uuid   NOT NULL REFERENCES app.rows(id) ON DELETE CASCADE,
  column_id bigint NOT NULL REFERENCES app.columns(id) ON DELETE CASCADE,
  value     uuid   NULL REFERENCES app.rows(id) ON DELETE RESTRICT,
  PRIMARY KEY (row_id, column_id)
);
