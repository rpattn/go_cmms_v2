// internal/auth/session.go
package auth

import (
    "context"
    "net/http"
    "time"
    "net/netip"
    "strings"

    "yourapp/internal/models"
    "yourapp/internal/session"

    "github.com/google/uuid"
)

type ctxKeyUser struct{}
type ctxKeySession struct{}

type ctxKey string

var (
	ctxOrg  ctxKey = "org"
	ctxSess ctxKey = "session"
)

// cookieSecure controls whether the session cookie is marked Secure.
// Default true; main() should override based on config for local dev.
var cookieSecure = true

// SetCookieSecurity allows main.go to configure whether cookies are Secure.
func SetCookieSecurity(secure bool) { cookieSecure = secure }

var sameSiteMode = http.SameSiteLaxMode

// SetCookieSameSite allows configuring SameSite mode: "lax", "none", "strict".
func SetCookieSameSite(mode string) {
    switch mode {
    case "none":
        sameSiteMode = http.SameSiteNoneMode
    case "strict":
        sameSiteMode = http.SameSiteStrictMode
    default:
        sameSiteMode = http.SameSiteLaxMode
    }
}

func SetSessionCookie(w http.ResponseWriter, s models.Session) {
    // Store server-side and set an opaque session id cookie
    sid := session.DefaultStore.Create(s)
    http.SetCookie(w, &http.Cookie{
        Name:     "session",
        Value:    sid,
        Path:     "/",
        HttpOnly: true,
        Secure:   cookieSecure,
        SameSite: sameSiteMode,
        Expires:  s.Expiry,
    })
}

func ReadSession(r *http.Request) *models.Session {
    c, err := r.Cookie("session")
    if err != nil || c.Value == "" {
        return nil
    }
    sess, ok := session.DefaultStore.Get(c.Value)
    if !ok {
        return nil
    }
    if !sess.Expiry.IsZero() && sess.Expiry.Before(time.Now()) {
        return nil
    }
    // Return a copy to avoid mutation of store by callers
    s := sess
    return &s
}

func OrgFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(ctxOrg).(uuid.UUID)
	return id, ok
}

func UserFromContext(ctx context.Context) (*models.User, bool) {
	val := ctx.Value(ctxKeyUser{})
	if val == nil {
		return nil, false
	}
	return val.(*models.User), true
}

func SessionFromContext(ctx context.Context) (*models.Session, bool) {
	val := ctx.Value(ctxSess)
	if val == nil {
		return nil, false
	}
	return val.(*models.Session), true
}

func WithUser(ctx context.Context, u *models.User) context.Context {
	return context.WithValue(ctx, ctxKeyUser{}, u)
}

func WithOrg(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, ctxOrg, id)
}

func GetUserFromContext(ctx context.Context) (*models.User, bool) {
	u, ok := ctx.Value(ctxKeyUser{}).(*models.User)
	return u, ok
}

func WithSession(ctx context.Context, s *models.Session) context.Context {
    // Set under both keys for compatibility
    ctx = context.WithValue(ctx, ctxKeySession{}, s)
    return context.WithValue(ctx, ctxSess, s)
}

func GetSessionFromContext(ctx context.Context) (*models.Session, bool) {
	s, ok := ctx.Value(ctxKeySession{}).(*models.Session)
	return s, ok
}

// clientIP extracts a best-effort client IP from headers or RemoteAddr.
func clientIP(r *http.Request) (netip.Addr, bool) {
    // Try common proxy header first
    if ff := r.Header.Get("X-Forwarded-For"); ff != "" {
        // XFF may be a list: client, proxy1, proxy2
        parts := strings.Split(ff, ",")
        if len(parts) > 0 {
            if ip, err := netip.ParseAddr(strings.TrimSpace(parts[0])); err == nil {
                return ip, true
            }
        }
    }
    // Fallback to X-Real-IP
    if rip := r.Header.Get("X-Real-IP"); rip != "" {
        if ip, err := netip.ParseAddr(strings.TrimSpace(rip)); err == nil {
            return ip, true
        }
    }
    // RemoteAddr may include port
    host := r.RemoteAddr
    if i := strings.LastIndex(host, ":"); i > 0 {
        host = host[:i]
    }
    if ip, err := netip.ParseAddr(host); err == nil {
        return ip, true
    }
    return netip.Addr{}, false
}
