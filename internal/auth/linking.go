package auth

import (
    "encoding/json"
    "net/http"
    "strings"
    "sync"
    "time"

    "yourapp/internal/config"
    "yourapp/internal/models"
    "yourapp/internal/repo"
)

// pendingLink represents an identity that needs password-based linking.
type pendingLink struct {
    Provider string
    Subject  string
    Email    string
    Name     string
    Picture  string
    Expires  time.Time
}

var (
    pendingMu sync.Mutex
    pendingMap = make(map[string]pendingLink)
)

func putPending(pl pendingLink, ttl time.Duration) string {
    if ttl <= 0 {
        ttl = 10 * time.Minute
    }
    token := randString(24)
    pl.Expires = time.Now().Add(ttl)
    pendingMu.Lock()
    pendingMap[token] = pl
    pendingMu.Unlock()
    return token
}

func getPending(token string) (pendingLink, bool) {
    pendingMu.Lock()
    defer pendingMu.Unlock()
    pl, ok := pendingMap[token]
    if !ok {
        return pendingLink{}, false
    }
    if !pl.Expires.IsZero() && pl.Expires.Before(time.Now()) {
        delete(pendingMap, token)
        return pendingLink{}, false
    }
    return pl, true
}

func delPending(token string) {
    pendingMu.Lock()
    delete(pendingMap, token)
    pendingMu.Unlock()
}

func setPendingCookie(w http.ResponseWriter, token string, ttl time.Duration) {
    if ttl <= 0 {
        ttl = 10 * time.Minute
    }
    http.SetCookie(w, &http.Cookie{
        Name:     "pending_link",
        Value:    token,
        Path:     "/",
        HttpOnly: true,
        Secure:   cookieSecure,
        SameSite: sameSiteMode,
        Expires:  time.Now().Add(ttl),
    })
}

func readPendingCookie(r *http.Request) string {
    if c, err := r.Cookie("pending_link"); err == nil && c != nil {
        return c.Value
    }
    return ""
}

// LinkWithPasswordHandler completes linking using a password check.
// POST /auth/link/password
// Body: { "email": "...", "password": "..." }
func LinkWithPasswordHandler(r repo.Repo, cfg config.Config) http.HandlerFunc {
    type reqBody struct {
        Email    string `json:"email"`
        Password string `json:"password"`
        TOTPCode string `json:"totp_code"`
    }
    return func(w http.ResponseWriter, req *http.Request) {
        token := readPendingCookie(req)
        if token == "" {
            writeJSON(w, http.StatusBadRequest, map[string]any{
                "error":   "no_pending_link",
                "message": "No pending identity to link",
            })
            return
        }
        pl, ok := getPending(token)
        if !ok {
            writeJSON(w, http.StatusBadRequest, map[string]any{
                "error":   "expired_pending_link",
                "message": "Pending link expired or missing",
            })
            return
        }

        var body reqBody
        if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
            writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad_json"})
            return
        }
        email := strings.ToLower(strings.TrimSpace(body.Email))
        if email == "" || strings.TrimSpace(body.Password) == "" {
            writeJSON(w, http.StatusBadRequest, map[string]any{"error": "missing_fields"})
            return
        }

        // Authenticate local credential by email (we store username as email by default)
        cred, user, err := r.GetLocalCredentialByUsername(req.Context(), email)
        if err != nil || !VerifyPassword(body.Password, cred.PasswordHash) {
            writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "invalid_login"})
            return
        }

        // If this local account has MFA enabled, enforce TOTP for linking
        if r.UserHasTOTP(req.Context(), user.ID) {
            sec, ok := r.GetTOTPSecret(req.Context(), user.ID)
            if !ok || strings.TrimSpace(body.TOTPCode) == "" || !validateTOTP(sec, body.TOTPCode) {
                writeJSON(w, http.StatusUnauthorized, map[string]any{
                    "error":   "invalid_mfa",
                    "message": "Valid two-factor code required to link",
                })
                return
            }
        }

        // Enforce email match to prevent cross-account linking
        if strings.ToLower(strings.TrimSpace(user.Email)) != strings.ToLower(strings.TrimSpace(pl.Email)) {
            writeJSON(w, http.StatusConflict, map[string]any{
                "error":   "email_mismatch",
                "message": "Authenticated account email does not match provider email",
            })
            return
        }

        // Ensure the identity isn't already linked to a different user
        if existing, err := r.GetUserByIdentity(req.Context(), pl.Provider, pl.Subject); err == nil {
            if existing.ID != user.ID {
                writeJSON(w, http.StatusConflict, map[string]any{"error": "identity_linked_elsewhere"})
                return
            }
            // already linked to same user; proceed to login
        }

        // Link identity
        if err := r.LinkIdentity(req.Context(), user.ID, pl.Provider, pl.Subject); err != nil {
            writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "link_failed"})
            return
        }

        // Save avatar/profile if available
        if strings.TrimSpace(pl.Picture) != "" {
            _ = r.UpdateUserProfile(req.Context(), user.ID, nil, &pl.Picture, nil, nil)
        }

        // Success: clear cookie and pending state
        delPending(token)
        http.SetCookie(w, &http.Cookie{
            Name:     "pending_link",
            Value:    "",
            Path:     "/",
            Expires:  time.Unix(0, 0),
            HttpOnly: true,
            Secure:   cookieSecure,
            SameSite: sameSiteMode,
        })

        // Log user in using existing org membership
        org, err := r.PickUserOrg(req.Context(), user.ID)
        if err != nil {
            writeJSON(w, http.StatusForbidden, map[string]any{"error": "no_org"})
            return
        }
        SetSessionCookie(w, models.Session{
            UserID:    user.ID,
            ActiveOrg: org.ID,
            Provider:  pl.Provider,
            Expiry:    time.Now().Add(8 * time.Hour),
        })

        // Record successful login after linking
        if ip, ok := clientIP(req); ok {
            _ = r.RecordLoginSuccess(req.Context(), user.Email, ip)
        }
        // Redirect to frontend if configured; otherwise fallback to current host
        if strings.TrimSpace(cfg.Frontend.URL) != "" {
            base := strings.TrimRight(cfg.Frontend.URL, "/")
            path := cfg.Frontend.PostLoginPath
            if strings.TrimSpace(path) == "" { path = "/app/work-orders" }
            path = "/" + strings.TrimLeft(path, "/")
            http.Redirect(w, req, base+path, http.StatusFound)
            return
        }
        scheme := req.Header.Get("X-Forwarded-Proto"); if scheme == "" { scheme = "http" }
        host := req.Header.Get("X-Forwarded-Host"); if host == "" { host = req.Host }
        http.Redirect(w, req, scheme+"://"+host+"/app/work-orders", http.StatusFound)
    }
}
