package auth

import (
    "encoding/json"
    "net/http"
    "strings"

    "yourapp/internal/repo"
)

// UpdateProfileHandler allows a logged-in user to update optional profile fields.
// PUT /auth/profile
// Body: { "name": "...", "phone": "...", "country": "...", "avatar_url": "..." }
func UpdateProfileHandler(r repo.Repo) http.HandlerFunc {
    type bodyT struct {
        Name      *string `json:"name"`
        Phone     *string `json:"phone"`
        Country   *string `json:"country"`
        AvatarURL *string `json:"avatar_url"`
    }
    return func(w http.ResponseWriter, req *http.Request) {
        sess := ReadSession(req)
        if sess == nil {
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }
        var b bodyT
        if err := json.NewDecoder(req.Body).Decode(&b); err != nil {
            http.Error(w, "bad json", http.StatusBadRequest)
            return
        }
        norm := func(p *string) *string {
            if p == nil { return nil }
            s := strings.TrimSpace(*p)
            return &s
        }
        if err := r.UpdateUserProfile(req.Context(), sess.UserID, norm(b.Name), norm(b.AvatarURL), norm(b.Phone), norm(b.Country)); err != nil {
            http.Error(w, "update failed", http.StatusInternalServerError)
            return
        }
        writeJSON(w, http.StatusOK, map[string]any{"ok": true})
    }
}

