package middleware

import (
    "context"
    "net/http"

    "yourapp/internal/auth"
)

// private context keys for logging enrichment
type ctxKey string

const (
    ctxLogUserID ctxKey = "log_user_id"
    ctxLogOrgID  ctxKey = "log_org_id"
    ctxLogProv   ctxKey = "log_provider"
)

// EnrichLogger stores user_id/org_id/provider into context for logging handlers to pick up.
func EnrichLogger(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        ctx := r.Context()
        if sess, ok := auth.SessionFromContext(ctx); ok && sess != nil {
            ctx = context.WithValue(ctx, ctxLogUserID, sess.UserID.String())
            ctx = context.WithValue(ctx, ctxLogOrgID, sess.ActiveOrg.String())
            if sess.Provider != "" {
                ctx = context.WithValue(ctx, ctxLogProv, sess.Provider)
            }
        } else if u, ok := auth.GetUserFromContext(ctx); ok && u != nil {
            ctx = context.WithValue(ctx, ctxLogUserID, u.ID.String())
        }
        if org, ok := auth.OrgFromContext(ctx); ok {
            ctx = context.WithValue(ctx, ctxLogOrgID, org.String())
        }
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

// GetLogUserID returns the enriched user id if set.
func GetLogUserID(ctx context.Context) (string, bool) {
    v, ok := ctx.Value(ctxLogUserID).(string)
    return v, ok && v != ""
}

// GetLogOrgID returns the enriched org id if set.
func GetLogOrgID(ctx context.Context) (string, bool) {
    v, ok := ctx.Value(ctxLogOrgID).(string)
    return v, ok && v != ""
}

// GetLogProvider returns the enriched provider if set.
func GetLogProvider(ctx context.Context) (string, bool) {
    v, ok := ctx.Value(ctxLogProv).(string)
    return v, ok && v != ""
}
