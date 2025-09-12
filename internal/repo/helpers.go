package repo

import (
    "time"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgtype"

    db "yourapp/internal/db/gen"
    "yourapp/internal/models"
)

// Common pg/uuid helpers
func fromUUID(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }
func toUUID(u pgtype.UUID) uuid.UUID   { return uuid.UUID(u.Bytes) }
func toPgUUID(id uuid.UUID) pgtype.UUID {
    return pgtype.UUID{Bytes: id, Valid: true}
}

// Text conversions
func toText(s string) pgtype.Text { return pgtype.Text{String: s, Valid: true} }
func fromText(t pgtype.Text) string { return t.String }
func toNullText(p *string) pgtype.Text {
    if p == nil { return pgtype.Text{} }
    return pgtype.Text{String: *p, Valid: true}
}

// toNullableText returns NULL when s is empty; otherwise a valid text.
func toNullableText(s string) pgtype.Text {
    if s == "" {
        return pgtype.Text{Valid: false}
    }
    return pgtype.Text{String: s, Valid: true}
}

// Role ranking utility (for best-match selection)
func rankRole(r string) int {
    switch r {
    case string(models.RoleOwner):
        return 4
    case string(models.RoleAdmin):
        return 3
    case string(models.RoleMember):
        return 2
    default:
        return 1 // Viewer or unknown
    }
}

// tiny helpers for pgtype.Text
func textOrEmpty(t pgtype.Text) string {
    if t.Valid {
        return t.String
    }
    return ""
}

// If your query doesn't return created_at columns, delete uses of extractTime/CreatedAt.
func extractTime(_ db.GetUserWithOrgAndRoleRow, _ string) *time.Time { return nil }
func zeroIfNil(t *time.Time) time.Time {
    if t == nil {
        return time.Time{}
    }
    return *t
}

// toTime tries to safely unwrap common timestamp representations into time.Time.
// Unknown or invalid inputs return zero time.
func toTime(v any) time.Time {
    switch x := v.(type) {
    case time.Time:
        return x
    case *time.Time:
        if x != nil {
            return *x
        }
    case pgtype.Timestamptz:
        if x.Valid {
            return x.Time
        }
    case pgtype.Timestamp:
        if x.Valid {
            return x.Time
        }
    }
    return time.Time{}
}
