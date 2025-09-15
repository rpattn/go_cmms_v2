package repo

import (
	"context"
	"encoding/json"
	"log/slog"

	db "yourapp/internal/db/gen"
	"yourapp/internal/models"
)

// SearchUserTable exposes the generic search over user-defined EAV tables.
func (p *pgRepo) SearchUserTable(ctx context.Context, table string, payload []byte) ([]models.TableRow, error) {
	slog.DebugContext(ctx, "SearchUserTable", "table", table)
	params := db.SearchUserTableParams{
		TableName: table,
		Payload:   payload,
	}
	rows, err := p.q.SearchUserTable(ctx, params)
	if err != nil {
		slog.ErrorContext(ctx, "SearchUserTable failed", "err", err)
		return nil, err
	}
	out := make([]models.TableRow, 0, len(rows))
	for _, r := range rows {
		var data map[string]any
		if len(r.Data) > 0 {
			if err := json.Unmarshal(r.Data, &data); err != nil {
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
