package admin

import (
    "net/http"
    "time"

    httpserver "yourapp/internal/http"
    "yourapp/internal/repo"
    "yourapp/internal/session"
    "yourapp/internal/auth"
    "yourapp/internal/models"
)

// ListSessionsHandler returns JSON of active sessions.
// Access: Auth required; only Admin/Owner of current org may access.
func ListSessionsHandler(r repo.Repo) http.HandlerFunc {
    return func(w http.ResponseWriter, req *http.Request) {
        // Must be authenticated
        sess := auth.ReadSession(req)
        if sess == nil {
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }
        // Must be Admin or Owner in active org
        role, err := r.GetRole(req.Context(), sess.ActiveOrg, sess.UserID)
        if err != nil || (role != models.RoleAdmin && role != models.RoleOwner) {
            http.Error(w, "forbidden", http.StatusForbidden)
            return
        }
        // Build response
        type item struct {
            ID        string    `json:"id"`
            UserID    string    `json:"user_id"`
            OrgID     string    `json:"org_id"`
            Provider  string    `json:"provider"`
            ExpiresAt time.Time `json:"expires_at"`
        }
        entries := session.DefaultStore.List()
        out := make([]item, 0, len(entries))
        for _, e := range entries {
            out = append(out, item{
                ID:        e.ID,
                UserID:    e.Session.UserID.String(),
                OrgID:     e.Session.ActiveOrg.String(),
                Provider:  e.Session.Provider,
                ExpiresAt: e.Session.Expiry,
            })
        }
        httpserver.JSON(w, http.StatusOK, out)
    }
}

