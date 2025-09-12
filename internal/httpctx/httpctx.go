package httpctx

import (
    "context"

    "github.com/google/uuid"
    "yourapp/internal/auth"
    "yourapp/internal/models"
)

// Session returns the session from context if available.
func Session(ctx context.Context) (*models.Session, bool) {
    return auth.SessionFromContext(ctx)
}

// User returns the user pointer from context if available.
func User(ctx context.Context) (*models.User, bool) {
    return auth.GetUserFromContext(ctx)
}

// UserID returns a user id from context from either session or user.
func UserID(ctx context.Context) (uuid.UUID, bool) {
    if s, ok := auth.SessionFromContext(ctx); ok && s != nil {
        return s.UserID, true
    }
    if u, ok := auth.GetUserFromContext(ctx); ok && u != nil {
        return u.ID, true
    }
    return uuid.Nil, false
}

// OrgID returns the active org id from context.
func OrgID(ctx context.Context) (uuid.UUID, bool) {
    return auth.OrgFromContext(ctx)
}

// IDs returns both user and org ids if present.
func IDs(ctx context.Context) (userID, orgID uuid.UUID, ok bool) {
    u, uok := UserID(ctx)
    o, ook := OrgID(ctx)
    return u, o, uok && ook
}

