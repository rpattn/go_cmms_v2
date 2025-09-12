// internal/auth/local.go
package auth

import (
    "context"
    "crypto/rand"
    "crypto/subtle"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "log"

    //"errors"
    "net/http"
    "regexp"
    "strings"
    "time"

	"yourapp/internal/models"
	"yourapp/internal/repo"
	"yourapp/internal/session"

	//"github.com/google/uuid"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/argon2"
)

// ---------- Public handlers (mount under /auth) ----------

// POST /auth/signup
// Body: { "email": "...", "username": "...", "name": "...", "password": "...", "org_slug": "acme" }
func SignupHandler(r repo.Repo) http.HandlerFunc {
    return func(w http.ResponseWriter, req *http.Request) {
        var body struct {
            Email    string `json:"email"`
            Username string `json:"username"`
            Name     string `json:"name"`
            Password string `json:"password"`
            OrgSlug  string `json:"org_slug"`
        }
        if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
            http.Error(w, "bad json", http.StatusBadRequest)
            return
        }
        email := strings.ToLower(strings.TrimSpace(body.Email))
        username := strings.ToLower(strings.TrimSpace(body.Username))
        if email == "" || body.Password == "" {
            http.Error(w, "missing fields", http.StatusBadRequest)
            return
        }
        if username == "" {
            username = email
        }
        // Enforce org slug provided and non-empty for signup
        slug := strings.ToLower(strings.TrimSpace(body.OrgSlug))
        if slug == "" {
            http.Error(w, "org_slug required", http.StatusBadRequest)
            return
        }
        if !isValidSlug(slug) {
            http.Error(w, "invalid org_slug (use 3-32 chars: lowercase letters, digits, hyphens; cannot start/end with '-')", http.StatusBadRequest)
            return
        }

        u, err := r.UpsertUserByVerifiedEmail(req.Context(), email, strings.TrimSpace(body.Name))
        if err != nil {
            http.Error(w, "user upsert failed", http.StatusInternalServerError)
            return
        }

		phc, err := HashPassword(body.Password, defaultArgonParams())
		if err != nil {
			http.Error(w, "hash error", http.StatusInternalServerError)
			return
		}
		if err := r.CreateLocalCredential(req.Context(), u.ID, username, phc); err != nil {
			http.Error(w, "username taken", http.StatusConflict)
			return
		}

        // Signup can create a brand new organisation with the given slug.
        // If the slug already exists, reject signup and ask for an invite flow.
        if _, err := r.FindOrgBySlug(req.Context(), slug); err == nil {
            // Org already exists
            writeJSON(w, http.StatusConflict, map[string]any{
                "error":   "org_exists",
                "message": "Organisation already exists. Ask an owner for an invite.",
            })
            return
        }
        // Create org (use slug as default display name)
        org, err := r.CreateOrg(req.Context(), slug, slug, "")
        if err != nil {
            http.Error(w, "org create failed", http.StatusInternalServerError)
            return
        }
        if _, err := r.EnsureMembership(req.Context(), org.ID, u.ID, models.RoleOwner); err != nil {
            http.Error(w, "membership failed", http.StatusInternalServerError)
            fmt.Println("membership failed:", err)
            return
        }

        SetSessionCookie(w, models.Session{
            UserID:    u.ID,
            ActiveOrg: org.ID,
            Provider:  "local",
            Expiry:    time.Now().Add(8 * time.Hour),
        })
        writeJSON(w, http.StatusCreated, map[string]any{"ok": true})
    }
}

// slug: 3-32 chars, lowercase alnum and hyphen, cannot start/end with hyphen
var slugRE = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{1,30}[a-z0-9])$`)

func isValidSlug(s string) bool {
    return slugRE.MatchString(s)
}

// POST /auth/login
// Body: { "username": "...", "password": "...", "totp_code": "123456" }
func LoginHandler(r repo.Repo) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		var body struct {
			Username string `json:"username"`
			Password string `json:"password"`
			TOTPCode string `json:"totp_code"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			log.Println("login decode error:", err)
			return
		}
		username := strings.ToLower(strings.TrimSpace(body.Username))
		if username == "" || body.Password == "" {
			http.Error(w, "invalid login", http.StatusUnauthorized)
			log.Println("login missing fields")
			return
		}

		cred, user, err := r.GetLocalCredentialByUsername(req.Context(), username)
		if err != nil {
			http.Error(w, "invalid login", http.StatusUnauthorized)
			log.Println("login get credential error:", err)
			return
		}
		if !VerifyPassword(body.Password, cred.PasswordHash) {
			http.Error(w, "invalid login", http.StatusUnauthorized)
			log.Println("login bad password for user:", username)
			if ip, ok := clientIP(req); ok {
				_ = r.RecordLoginFailure(req.Context(), username, ip)
			}
			return
		}

        // If TOTP is enabled, enforce it
        if r.UserHasTOTP(req.Context(), user.ID) {
            if strings.TrimSpace(body.TOTPCode) == "" {
                writeJSON(w, http.StatusUnauthorized, map[string]any{
                    "error":   "mfa_required",
                    "message": "Two-factor code required",
                })
                log.Println("login missing mfa code for user:", username)
                return
            }
            sec, ok := r.GetTOTPSecret(req.Context(), user.ID)
            if !ok || !validateTOTP(sec, body.TOTPCode) {
                writeJSON(w, http.StatusUnauthorized, map[string]any{
                    "error":   "invalid_mfa",
                    "message": "Invalid two-factor code",
                })
                log.Println("login invalid mfa code for user:", username)
                return
            }
        }

		// Pick an org
		org, err := r.PickUserOrg(req.Context(), user.ID)
		if err != nil {
			http.Error(w, "no organisation", http.StatusForbidden)
			log.Println("login no org for user:", username)
			return
		}

		SetSessionCookie(w, models.Session{
			UserID:    user.ID,
			ActiveOrg: org.ID,
			Provider:  "local",
			Expiry:    time.Now().Add(8 * time.Hour),
		})

		// Record successful local login
		if ip, ok := clientIP(req); ok {
			_ = r.RecordLoginSuccess(req.Context(), username, ip)
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}

// POST /auth/logout
func LogoutHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// Best-effort delete server-side session
		if c, err := req.Cookie("session"); err == nil && c.Value != "" {
			session.DefaultStore.Delete(c.Value)
		}
        http.SetCookie(w, &http.Cookie{
            Name:     "session",
            Value:    "",
            Path:     "/",
            Expires:  time.Unix(0, 0),
            HttpOnly: true,
            Secure:   cookieSecure,
            SameSite: sameSiteMode,
        })
        w.WriteHeader(http.StatusNoContent)
    }
}

// GET /auth/mfa/totp/setup  -> returns { otpauth_url, secret }
func TOTPSetupBeginHandler(r repo.Repo) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		sess := ReadSession(req)
		if sess == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// For label we prefer the email; fetch user to get stable label if needed.
		// If your repo exposes GetUserByID, use it. Otherwise use a generic label.
		label := sess.UserID.String()
		key, err := totp.Generate(totp.GenerateOpts{
			Issuer:      "YourApp",
			AccountName: label,
			Period:      30,
			Digits:      otp.DigitsSix,
			Algorithm:   otp.AlgorithmSHA1, // Google Authenticator-compatible
		})
		if err != nil {
			http.Error(w, "totp error", http.StatusInternalServerError)
			return
		}
		if err := r.SetTOTPSecret(req.Context(), sess.UserID, key.Secret(), "YourApp", label); err != nil {
			http.Error(w, "store totp error", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{
			"otpauth_url": key.URL(),
			"secret":      key.Secret(),
		})
	}
}

// POST /auth/mfa/totp/verify  Body: { "code": "123456" }
func TOTPSetupVerifyHandler(r repo.Repo) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		sess := ReadSession(req)
		if sess == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var body struct {
			Code string `json:"code"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil || strings.TrimSpace(body.Code) == "" {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		secret, ok := r.GetTOTPSecret(req.Context(), sess.UserID)
		if !ok {
			http.Error(w, "no totp setup", http.StatusBadRequest)
			return
		}
		if !validateTOTP(secret, body.Code) {
			http.Error(w, "invalid code", http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}

// ---------- Helpers (org signup, json, crypto) ----------

func resolveOrgForSignup(ctx context.Context, r repo.Repo, slug, email string) (models.Org, error) {
	if s := strings.TrimSpace(slug); s != "" {
		return r.FindOrgBySlug(ctx, s)
	}
	// email domain -> slug
	if at := strings.LastIndex(email, "@"); at > 0 && at < len(email)-1 {
		domain := email[at+1:]
		if dot := strings.Index(domain, "."); dot > 0 {
			if org, err := r.FindOrgBySlug(ctx, strings.ToLower(domain[:dot])); err == nil {
				return org, nil
			}
		}
	}
	// fallback
	return r.FindOrgBySlug(ctx, "acme")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// Params
type ArgonParams struct {
	Memory  uint32 // KiB
	Time    uint32
	Threads uint8
	SaltLen uint32
	KeyLen  uint32
}

func defaultArgonParams() ArgonParams {
	return ArgonParams{
		Memory:  64 * 1024, // 64 MiB
		Time:    1,
		Threads: 4,
		SaltLen: 16,
		// 32-byte key is standard
		KeyLen: 32,
	}
}

// HashPassword returns a PHC string: $argon2id$v=19$m=...,t=...,p=...$salt$hash
func HashPassword(pw string, p ArgonParams) (string, error) {
	if strings.TrimSpace(pw) == "" {
		return "", fmt.Errorf("empty password")
	}
	salt := make([]byte, p.SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(pw), salt, p.Time, p.Memory, p.Threads, p.KeyLen)
	return phcEncode(p, salt, key), nil
}

// VerifyPassword checks a plaintext password against a PHC-encoded argon2id hash.
func VerifyPassword(pw, phc string) bool {
	mem, timeCost, threads, salt, want, ok := phcParse(phc)
	if !ok {
		return false
	}
	got := argon2.IDKey([]byte(pw), salt, timeCost, mem, threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// phcEncode builds the PHC string for argon2id.
func phcEncode(p ArgonParams, salt, key []byte) string {
	return fmt.Sprintf(
		"$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		p.Memory, p.Time, p.Threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	)
}

// phcParse extracts parameters, salt, and key from a PHC string.
// Returns (memoryKiB, time, threads, salt, key, ok).
func phcParse(phc string) (uint32, uint32, uint8, []byte, []byte, bool) {
	parts := strings.Split(phc, "$")
	// ["", "argon2id", "v=19", "m=...,t=...,p=...", "<saltB64>", "<keyB64>"]
	if len(parts) != 6 || parts[1] != "argon2id" || !strings.HasPrefix(parts[2], "v=") {
		return 0, 0, 0, nil, nil, false
	}
	var m, t, p int
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return 0, 0, 0, nil, nil, false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return 0, 0, 0, nil, nil, false
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return 0, 0, 0, nil, nil, false
	}
	return uint32(m), uint32(t), uint8(p), salt, key, true
}

// internal/auth/local.go — replace your validateTOTP with this
func validateTOTP(secret, code string) bool {
	// Quick check
	if totp.Validate(code, secret) {
		return true
	}
	// Allow small clock skew
	ok, _ := totp.ValidateCustom(code, secret, time.Now(), totp.ValidateOpts{
		Period:    30,
		Skew:      1,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	return ok
}

// ----- small rand helper -----

func randString(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
