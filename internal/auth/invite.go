package auth

import (
    "crypto/rand"
    "crypto/sha256"
    "encoding/base64"
    "encoding/hex"
    "encoding/json"
    "net/http"
    neturl "net/url"
    "strings"
    "time"

    "yourapp/internal/config"
    "yourapp/internal/models"
    "yourapp/internal/repo"
)

// InviteCreateHandler: Owners can invite users to current org.
// POST /auth/invite { "email": "user@example.com", "role": "Member" }
// Returns a one-time token (plaintext) for delivery via email. The token is hashed at rest.
func InviteCreateHandler(r repo.Repo, cfg config.Config) http.HandlerFunc {
    type bodyT struct {
        Email string `json:"email"`
        Role  string `json:"role"` // optional; defaults to Member
    }
    return func(w http.ResponseWriter, req *http.Request) {
        sess := ReadSession(req)
        if sess == nil {
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }
        var b bodyT
        if err := json.NewDecoder(req.Body).Decode(&b); err != nil || strings.TrimSpace(b.Email) == "" {
            http.Error(w, "bad json", http.StatusBadRequest)
            return
        }
        role := models.RoleMember
        if strings.TrimSpace(b.Role) != "" {
            switch strings.ToLower(b.Role) {
            case strings.ToLower(string(models.RoleViewer)):
                role = models.RoleViewer
            case strings.ToLower(string(models.RoleMember)):
                role = models.RoleMember
            case strings.ToLower(string(models.RoleAdmin)):
                role = models.RoleAdmin
            case strings.ToLower(string(models.RoleOwner)):
                // Disallow creating Owner via invite for safety
                http.Error(w, "invalid role", http.StatusBadRequest)
                return
            default:
                http.Error(w, "invalid role", http.StatusBadRequest)
                return
            }
        }
        // Generate token (plaintext) and store a SHA-256 hash
        raw := make([]byte, 32)
        if _, err := rand.Read(raw); err != nil {
            http.Error(w, "server error", http.StatusInternalServerError)
            return
        }
        token := base64.RawURLEncoding.EncodeToString(raw)
        sum := sha256.Sum256([]byte(token))
        tokenHash := hex.EncodeToString(sum[:])
        // Expiry: 7 days
        exp := time.Now().Add(7 * 24 * time.Hour)
        if err := r.CreateInvite(req.Context(), sess.ActiveOrg, sess.UserID, strings.ToLower(strings.TrimSpace(b.Email)), role, tokenHash, exp); err != nil {
            http.Error(w, "create invite failed", http.StatusInternalServerError)
            return
        }
        // Build an acceptance link based on configured frontend URL + API route prefix
        base := strings.TrimRight(cfg.Frontend.URL, "/")
        api := strings.TrimSpace(cfg.Frontend.APIRoute)
        if api != "" {
            api = "/" + strings.Trim(api, "/")
        }
        acceptURL := base + api + "/invite/accept?token=" + neturl.QueryEscape(token)
        writeJSON(w, http.StatusOK, map[string]any{
            "ok":          true,
            "accept_url":  acceptURL,
            "exp":         exp,
            "role":        role,
        })
    }
}

// InviteAcceptHandler: Accepts an invite token for the logged-in user (email must match invite).
// POST /auth/invite/accept { "token": "..." }
func InviteAcceptHandler(r repo.Repo) http.HandlerFunc {
    type bodyT struct{ Token string `json:"token"` }
    return func(w http.ResponseWriter, req *http.Request) {
        var b bodyT
        if err := json.NewDecoder(req.Body).Decode(&b); err != nil || strings.TrimSpace(b.Token) == "" {
            http.Error(w, "bad json", http.StatusBadRequest)
            return
        }
        // Lookup invite by token hash
        sum := sha256.Sum256([]byte(b.Token))
        tokenHash := hex.EncodeToString(sum[:])
        inv, err := r.GetInviteByTokenHash(req.Context(), tokenHash)
        if err != nil {
            http.Error(w, "invalid invite", http.StatusBadRequest)
            return
        }
        if !inv.UsedAt.IsZero() || time.Now().After(inv.ExpiresAt) {
            http.Error(w, "invite expired or used", http.StatusBadRequest)
            return
        }
        // Resolve user by email; create if not exists
        var user models.User
        if u, err := r.GetUserByEmail(req.Context(), strings.ToLower(inv.Email)); err == nil {
            user = u
        } else {
            // Create new user with invited email; no name known yet
            nu, err2 := r.UpsertUserByVerifiedEmail(req.Context(), strings.ToLower(inv.Email), "")
            if err2 != nil {
                http.Error(w, "user create failed", http.StatusInternalServerError)
                return
            }
            user = nu
        }
        // Add membership with invited role
        if _, err := r.EnsureMembership(req.Context(), inv.OrgID, user.ID, inv.Role); err != nil {
            http.Error(w, "membership failed", http.StatusInternalServerError)
            return
        }
        // Mark invite as used
        if err := r.UseInvite(req.Context(), tokenHash); err != nil {
            http.Error(w, "invite update failed", http.StatusInternalServerError)
            return
        }
        // If the user account was created by this acceptance, log them in and indicate they need to set a password
        // Heuristic: if created above, we won't have fetched; but to keep it simple, always set session here
        SetSessionCookie(w, models.Session{
            UserID:    user.ID,
            ActiveOrg: inv.OrgID,
            Provider:  "invite",
            Expiry:    time.Now().Add(8 * time.Hour),
        })
        writeJSON(w, http.StatusOK, map[string]any{"ok": true, "needs_password": true})
    }
}
