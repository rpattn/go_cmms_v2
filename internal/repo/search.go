package repo

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	db "yourapp/internal/db/gen"
	"yourapp/internal/models"
)

// ---------------- Search ----------------

func (p *pgRepo) SearchUsers(ctx context.Context, org_id uuid.UUID, payload []byte) ([]models.User, error) {
	slog.DebugContext(ctx, "SearchUsers", "org_id", org_id.String())
	params := db.SearchOrgUsersParams{
		OrgID:   fromUUID(org_id),
		Payload: payload,
	}
	rows, err := p.q.SearchOrgUsers(ctx, params)
	if err != nil {
		slog.ErrorContext(ctx, "SearchUsers failed", "err", err)
		return nil, err
	}
	if len(rows) == 0 {
		return []models.User{}, nil
	}
	users := make([]models.User, 0, len(rows))
	for _, r := range rows {
		u := models.User{
			ID:    toUUID(r.ID),
			Email: r.Email,
			Name:  r.Name,
		}
		users = append(users, u)
	}
	slog.DebugContext(ctx, "SearchUsers ok", "count", len(users))
	return users, nil
}
