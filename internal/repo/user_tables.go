package repo

import (
    "context"
    "encoding/json"
    "log/slog"
    "time"

    db "yourapp/internal/db/gen"
    "yourapp/internal/models"

    "github.com/google/uuid"
)

// toJSONBytes tries to normalize various JSON representations to a []byte
func toJSONBytes(v any) []byte {
    switch x := v.(type) {
    case []byte:
        return x
    case json.RawMessage:
        return []byte(x)
    case string:
        return []byte(x)
    default:
        b, _ := json.Marshal(x)
        return b
    }
}

func toString(v any) string {
    switch x := v.(type) {
    case nil:
        return ""
    case string:
        return x
    case []byte:
        return string(x)
    default:
        b, _ := json.Marshal(x)
        if len(b) > 0 && b[0] == '"' && b[len(b)-1] == '"' {
            var s string
            if err := json.Unmarshal(b, &s); err == nil { return s }
        }
        return string(b)
    }
}

// SearchUserTable exposes the generic search over user-defined EAV tables.
func (p *pgRepo) SearchUserTable(ctx context.Context, org_id uuid.UUID, table string, payload []byte) ([]models.TableRow, error) {
	slog.DebugContext(ctx, "SearchUserTable", "org_id", org_id.String(), "table", table)
	params := db.SearchUserTableParams{
		TableName: table,
		Payload:   payload,
		OrgID:     fromUUID(org_id),
	}
	rows, err := p.q.SearchUserTable(ctx, params)
	if err != nil {
		slog.ErrorContext(ctx, "SearchUserTable failed", "err", err)
		return nil, err
	}
	out := make([]models.TableRow, 0, len(rows))
	for _, r := range rows {
        var data map[string]any
        if b := toJSONBytes(r.Data); len(b) > 0 {
            if err := json.Unmarshal(b, &data); err != nil {
                // If malformed row JSON, still return row with empty data rather than failing whole page
                slog.WarnContext(ctx, "SearchUserTable: bad row JSON", "err", err)
            }
        }
		out = append(out, models.TableRow{
			RowID:      toUUID(r.RowID),
			Data:       data,
			TotalCount: r.TotalCount,
		})
	}
	slog.DebugContext(ctx, "SearchUserTable ok", "count", len(out))
	return out, nil
}

// GetUserTableSchema returns the list of columns for a user-defined table in an org.
func (p *pgRepo) GetUserTableSchema(ctx context.Context, org_id uuid.UUID, table string) ([]models.TableColumn, error) {
	slog.DebugContext(ctx, "GetUserTableSchema", "org_id", org_id.String(), "table", table)
	rows, err := p.q.GetUserTableSchema(ctx, db.GetUserTableSchemaParams{
		TableName: table,
		OrgID:     fromUUID(org_id),
	})
	if err != nil {
		slog.ErrorContext(ctx, "GetUserTableSchema failed", "err", err)
		return nil, err
	}
	out := make([]models.TableColumn, 0, len(rows))
	for _, r := range rows {
		var enums []string
		if len(r.EnumValues) > 0 {
			if err := json.Unmarshal(r.EnumValues, &enums); err != nil {
				slog.WarnContext(ctx, "GetUserTableSchema: bad enum_values JSON", "err", err)
			}
		}
		var refID *int64
		if r.ReferenceTableID.Valid {
			v := r.ReferenceTableID.Int64
			refID = &v
		}
		out = append(out, models.TableColumn{
			ID:                    r.ID,
			Name:                  r.Name,
			Type:                  r.Type,
			Required:              r.IsRequired,
			Indexed:               r.IsIndexed,
			EnumValues:            enums,
			IsReference:           r.IsReference,
			ReferenceTableID:      refID,
			RequireDifferentTable: r.RequireDifferentTable,
		})
	}
	return out, nil
}

func (p *pgRepo) CreateUserTable(ctx context.Context, orgID uuid.UUID, name string) (models.UserTable, bool, error) {
	slog.DebugContext(ctx, "CreateUserTable", "org_id", orgID.String(), "name", name)
	row, err := p.q.CreateUserTable(ctx, db.CreateUserTableParams{
		OrgID: fromUUID(orgID),
		Name:  name,
	})
	if err != nil {
		slog.ErrorContext(ctx, "CreateUserTable failed", "err", err)
		return models.UserTable{}, false, err
	}
	ut := models.UserTable{
		ID:   row.ID,
		Name: row.Name,
		Slug: row.Slug,
		CreatedAt: func() time.Time {
			if row.CreatedAt.Valid {
				return row.CreatedAt.Time
			}
			return time.Now()
		}(),
	}
	return ut, row.Created, nil
}

func (p *pgRepo) ListUserTables(ctx context.Context, orgID uuid.UUID) ([]models.UserTable, error) {
	slog.DebugContext(ctx, "ListUserTables", "org_id", orgID.String())
	rows, err := p.q.ListUserTables(ctx, fromUUID(orgID))
	if err != nil {
		slog.ErrorContext(ctx, "ListUserTables failed", "err", err)
		return nil, err
	}
	out := make([]models.UserTable, 0, len(rows))
	for _, r := range rows {
		created := time.Time{}
		if r.CreatedAt.Valid {
			created = r.CreatedAt.Time
		}
		out = append(out, models.UserTable{
			ID:        r.ID,
			Name:      r.Name,
			Slug:      r.Slug,
			CreatedAt: created,
		})
	}
	return out, nil
}

func (p *pgRepo) DeleteUserTable(ctx context.Context, orgID uuid.UUID, table string) (models.UserTable, bool, error) {
	slog.DebugContext(ctx, "DeleteUserTable", "org_id", orgID.String(), "table", table)
	row, err := p.q.DeleteUserTable(ctx, db.DeleteUserTableParams{
		OrgID:     fromUUID(orgID),
		TableName: table,
	})
	if err != nil {
		slog.ErrorContext(ctx, "DeleteUserTable failed", "err", err)
		return models.UserTable{}, false, err
	}
	if !row.Deleted {
		return models.UserTable{}, false, nil
	}
	created := time.Time{}
	if row.CreatedAt.Valid {
		created = row.CreatedAt.Time
	}
	ut := models.UserTable{
		ID:        row.ID,
		Name:      row.Name,
		Slug:      row.Slug,
		CreatedAt: created,
	}
	return ut, true, nil
}

func (p *pgRepo) AddUserTableColumn(ctx context.Context, orgID uuid.UUID, table string, input models.TableColumnInput) (models.TableColumn, bool, error) {
	slog.DebugContext(ctx, "AddUserTableColumn", "org_id", orgID.String(), "table", table, "name", input.Name)
	// Marshal enum values to JSON for the query
	var enumJSON []byte
	if len(input.EnumValues) > 0 {
		b, err := json.Marshal(input.EnumValues)
		if err != nil {
			slog.ErrorContext(ctx, "AddUserTableColumn: bad enum values", "err", err)
			return models.TableColumn{}, false, err
		}
		enumJSON = b
	}
	row, err := p.q.AddUserTableColumn(ctx, db.AddUserTableColumnParams{
		OrgID:                 fromUUID(orgID),
		TableName:             table,
		ColumnName:            input.Name,
		ColType:               input.Type,
		IsRequired:            input.Required,
		IsIndexed:             input.Indexed,
		EnumValues:            enumJSON,
		IsReference:           input.IsReference,
		ReferenceTable:        input.ReferenceTable,
		RequireDifferentTable: input.RequireDifferentTable,
	})
	if err != nil {
		slog.ErrorContext(ctx, "AddUserTableColumn failed", "err", err)
		return models.TableColumn{}, false, err
	}
	var enums []string
	if len(row.EnumValues) > 0 {
		if err := json.Unmarshal(row.EnumValues, &enums); err != nil {
			slog.WarnContext(ctx, "AddUserTableColumn: bad enum_values JSON from DB", "err", err)
		}
	}
	var refID *int64
	if row.ReferenceTableID.Valid {
		v := row.ReferenceTableID.Int64
		refID = &v
	}
	col := models.TableColumn{
		ID:                    row.ID,
		Name:                  row.Name,
		Type:                  row.Type,
		Required:              row.IsRequired,
		Indexed:               row.IsIndexed,
		EnumValues:            enums,
		IsReference:           row.IsReference,
		ReferenceTableID:      refID,
		RequireDifferentTable: row.RequireDifferentTable,
	}
	return col, row.Created, nil
}

func (p *pgRepo) RemoveUserTableColumn(ctx context.Context, orgID uuid.UUID, table string, columnName string) (models.TableColumn, bool, error) {
	slog.DebugContext(ctx, "RemoveUserTableColumn", "org_id", orgID.String(), "table", table, "column", columnName)
	row, err := p.q.RemoveUserTableColumn(ctx, db.RemoveUserTableColumnParams{
		OrgID:      fromUUID(orgID),
		TableName:  table,
		ColumnName: columnName,
	})
	if err != nil {
		slog.ErrorContext(ctx, "RemoveUserTableColumn failed", "err", err)
		return models.TableColumn{}, false, err
	}
	// If nothing was deleted, treat as not found
	if !row.Deleted {
		return models.TableColumn{}, false, nil
	}
	var enums []string
	if len(row.EnumValues) > 0 {
		if err := json.Unmarshal(row.EnumValues, &enums); err != nil {
			slog.WarnContext(ctx, "RemoveUserTableColumn: bad enum_values JSON from DB", "err", err)
		}
	}
	var refID *int64
	if row.ReferenceTableID.Valid {
		v := row.ReferenceTableID.Int64
		refID = &v
	}
	col := models.TableColumn{
		ID:                    row.ID,
		Name:                  row.Name,
		Type:                  row.Type,
		Required:              row.IsRequired,
		Indexed:               row.IsIndexed,
		EnumValues:            enums,
		IsReference:           row.IsReference,
		ReferenceTableID:      refID,
		RequireDifferentTable: row.RequireDifferentTable,
	}
	return col, row.Deleted, nil
}

func (p *pgRepo) InsertUserTableRow(ctx context.Context, orgID uuid.UUID, table string, values []byte) (models.TableRow, error) {
	slog.DebugContext(ctx, "InsertUserTableRow", "org_id", orgID.String(), "table", table)
	row, err := p.q.InsertUserTableRow(ctx, db.InsertUserTableRowParams{
		OrgID:     fromUUID(orgID),
		TableName: table,
		Values:    values,
	})
	if err != nil {
		slog.ErrorContext(ctx, "InsertUserTableRow failed", "err", err)
		return models.TableRow{}, err
	}
    var data map[string]any
    if b := toJSONBytes(row.Data); len(b) > 0 {
        if err := json.Unmarshal(b, &data); err != nil {
            slog.WarnContext(ctx, "InsertUserTableRow: bad row JSON", "err", err)
        }
    }
	return models.TableRow{
		RowID:      toUUID(row.RowID),
		Data:       data,
		TotalCount: 0,
	}, nil
}

func (p *pgRepo) GetRowData(ctx context.Context, orgID uuid.UUID, rowID uuid.UUID) (map[string]any, bool, error) {
	slog.DebugContext(ctx, "GetRowData", "org_id", orgID.String(), "row_id", rowID.String())
	r, err := p.q.GetRowData(ctx, db.GetRowDataParams{
		OrgID: toPgUUID(orgID),
		RowID: toPgUUID(rowID),
	})
	if err != nil {
		slog.ErrorContext(ctx, "GetRowData failed", "err", err)
		return nil, false, err
	}
	if !r.Found {
		return nil, false, nil
	}
    var data map[string]any
    if b := toJSONBytes(r.Data); len(b) > 0 {
        if err := json.Unmarshal(b, &data); err != nil {
            slog.WarnContext(ctx, "GetRowData: bad row JSON", "err", err)
        }
    }
	return data, true, nil
}

func (p *pgRepo) LookupIndexedRows(ctx context.Context, orgID uuid.UUID, table string, field *string, q *string, limit int) ([]models.IndexedRow, error) {
	slog.DebugContext(ctx, "LookupIndexedRows", "org_id", orgID.String(), "table", table, "field", ptrStr(field), "q", ptrStr(q), "limit", limit)
	var fld, qq string
	if field != nil {
		fld = *field
	}
	if q != nil {
		qq = *q
	}
	if limit <= 0 {
		limit = 20
	}
	rows, err := p.q.LookupIndexedRows(ctx, db.LookupIndexedRowsParams{
		OrgID:      fromUUID(orgID),
		TableName:  table,
		Field:      fld,
		Q:          qq,
		LimitCount: int32(limit),
	})
	if err != nil {
		slog.ErrorContext(ctx, "LookupIndexedRows failed", "err", err)
		return nil, err
	}
	out := make([]models.IndexedRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, models.IndexedRow{ID: toUUID(r.RowID), Label: r.Label.String})
	}
	return out, nil
}

func ptrStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func (p *pgRepo) ListIndexedFields(ctx context.Context, orgID uuid.UUID) ([]models.IndexedField, error) {
	slog.DebugContext(ctx, "ListIndexedFields", "org_id", orgID.String())
	rows, err := p.q.ListIndexedColumns(ctx, fromUUID(orgID))
	if err != nil {
		slog.ErrorContext(ctx, "ListIndexedFields failed", "err", err)
		return nil, err
	}
	out := make([]models.IndexedField, 0, len(rows))
	for _, r := range rows {
		out = append(out, models.IndexedField{
			TableID:    r.TableID,
			TableSlug:  r.TableSlug,
			TableName:  r.TableName,
			ColumnID:   r.ColumnID,
			ColumnName: r.ColumnName,
			ColumnType: r.ColumnType,
		})
	}
	return out, nil
}

func (p *pgRepo) DeleteUserTableRow(ctx context.Context, orgID uuid.UUID, table string, rowID uuid.UUID) (bool, error) {
	slog.DebugContext(ctx, "DeleteUserTableRow", "org_id", orgID.String(), "table", table, "row_id", rowID.String())
	r, err := p.q.DeleteUserTableRow(ctx, db.DeleteUserTableRowParams{
		OrgID:     fromUUID(orgID),
		TableName: table,
		RowID:     fromUUID(rowID),
	})
	if err != nil {
		slog.ErrorContext(ctx, "DeleteUserTableRow failed", "err", err)
		return false, err
	}
	return r.Deleted, nil
}

func (p *pgRepo) GetRowLabel(ctx context.Context, orgID uuid.UUID, tableID int64, rowID uuid.UUID) (string, error) {
    lbl, err := p.q.GetRowLabel(ctx, db.GetRowLabelParams{
        OrgID:   fromUUID(orgID),
        RowID:   fromUUID(rowID),
        TableID: tableID,
    })
    if err != nil {
        return "", err
    }
    return toString(lbl), nil
}

func (p *pgRepo) GetRowLabelAuto(ctx context.Context, orgID uuid.UUID, rowID uuid.UUID) (string, error) {
    lbl, err := p.q.GetRowLabelAuto(ctx, db.GetRowLabelAutoParams{
        OrgID: fromUUID(orgID),
        RowID: fromUUID(rowID),
    })
    if err != nil {
        return "", err
    }
    return toString(lbl), nil
}

func (p *pgRepo) BatchGetRowLabels(ctx context.Context, orgID uuid.UUID, tableID int64, rowIDs []uuid.UUID) (map[uuid.UUID]string, error) {
    // Build JSON array of UUID strings
    arr := make([]string, 0, len(rowIDs))
    for _, id := range rowIDs { arr = append(arr, id.String()) }
    b, _ := json.Marshal(arr)
    rows, err := p.q.BatchGetRowLabels(ctx, db.BatchGetRowLabelsParams{
        OrgID:   fromUUID(orgID),
        TableID: tableID,
        Ids:     b,
    })
    if err != nil { return nil, err }
    out := make(map[uuid.UUID]string, len(rows))
    for _, r := range rows {
        out[toUUID(r.RowID)] = toString(r.Label)
    }
    return out, nil
}

func (p *pgRepo) BatchGetRowLabelsAuto(ctx context.Context, orgID uuid.UUID, rowIDs []uuid.UUID) (map[uuid.UUID]string, error) {
    arr := make([]string, 0, len(rowIDs))
    for _, id := range rowIDs { arr = append(arr, id.String()) }
    b, _ := json.Marshal(arr)
    rows, err := p.q.BatchGetRowLabelsAuto(ctx, db.BatchGetRowLabelsAutoParams{
        OrgID: fromUUID(orgID),
        Ids:   b,
    })
    if err != nil { return nil, err }
    out := make(map[uuid.UUID]string, len(rows))
    for _, r := range rows {
        out[toUUID(r.RowID)] = toString(r.Label)
    }
    return out, nil
}
