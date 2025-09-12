// middleware/context.go
package middleware

import (
	"context"

	"yourapp/internal/auth"
	"yourapp/internal/models"
)

// Keep your existing helpers...
func WithSession(ctx context.Context, s *models.Session) context.Context {
	return auth.WithSession(ctx, s)
}
func GetSessionFromContext(ctx context.Context) (*models.Session, bool) {
	return auth.GetSessionFromContext(ctx)
}
