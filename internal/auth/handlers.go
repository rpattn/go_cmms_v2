// internal/auth/handlers.go
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"yourapp/internal/config"
	"yourapp/internal/models"
	"yourapp/internal/repo"
	"yourapp/internal/security"

	//"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-chi/chi/v5"
	//"github.com/google/uuid"
	"golang.org/x/oauth2"
)

// --- Cookies for state/nonce ---

const (
	cookieState = "oauth_state"
	cookieNonce = "oidc_nonce"
)

// --- Public handlers (mount these in your router) ---

// StartHandler: GET /auth/{provider}
func StartHandler(providers map[ProviderKind]*Provider, r repo.Repo) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		pname := ProviderKind(chi.URLParam(req, "provider"))
		p, ok := providers[pname]
		if !ok || p == nil || p.OAuth2Config == nil {
			http.NotFound(w, req)
			return
		}

		state := randString(24)
		nonce := randString(24)

		setTempCookie(w, cookieState, state, 10*time.Minute)
		setTempCookie(w, cookieNonce, nonce, 10*time.Minute)

		var opts []oauth2.AuthCodeOption
		if p.OIDCVerifier != nil {
			opts = append(opts, oauth2.SetAuthURLParam("nonce", nonce))
		}
		http.Redirect(w, req, p.OAuth2Config.AuthCodeURL(state, opts...), http.StatusFound)
	}
}

// CallbackHandler: GET /auth/{provider}/callback
func CallbackHandler(providers map[ProviderKind]*Provider, r repo.Repo, cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		pname := ProviderKind(chi.URLParam(req, "provider"))
		p, ok := providers[pname]
		if !ok || p == nil || p.OAuth2Config == nil {
			http.NotFound(w, req)
			return
		}

		// CSRF
		if req.URL.Query().Get("state") != readCookie(req, cookieState) {
			http.Error(w, "bad state", http.StatusBadRequest)
			return
		}

		// Exchange code
		code := req.URL.Query().Get("code")
		tok, err := p.OAuth2Config.Exchange(ctx, code)
		if err != nil {
			http.Error(w, "exchange failed: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Extract identity (OIDC or OAuth2)
		id, err := extractIdentity(ctx, pname, p, tok, readCookie(req, cookieNonce))
		if err != nil {
			http.Error(w, "identity error: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Denylist check for provider+subject
		if security.IsIdentityDenied(string(pname), id.Subject) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		// Normalize email for matching
		id.Email = strings.ToLower(strings.TrimSpace(id.Email))

		// First, try existing identity link
		var (
			u              models.User
			existingLinked bool
		)
		if existing, err := r.GetUserByIdentity(ctx, string(pname), id.Subject); err == nil {
			u = existing
			existingLinked = true
		} else {
			// No identity link yet. If email belongs to an existing user, do NOT auto-link.
			if _, err2 := r.GetUserByEmail(ctx, id.Email); err2 == nil {
				// Security: require explicit account linking. Create a temporary pending link and set cookie.
				tok := putPending(pendingLink{Provider: string(pname), Subject: id.Subject, Email: id.Email, Name: id.Name, Picture: id.Picture}, 10*time.Minute)
				setPendingCookie(w, tok, 10*time.Minute)

				// If a frontend is configured, redirect the browser to a client route to complete linking.
				if strings.TrimSpace(cfg.Frontend.URL) != "" {
					base := strings.TrimRight(cfg.Frontend.URL, "/")
					dest := base + "/auth/link?reason=link_required&provider=" + url.QueryEscape(string(pname))
					http.Redirect(w, req, dest, http.StatusFound)
					return
				}
				// Otherwise return an API response (for non-browser clients)
				writeJSON(w, http.StatusConflict, map[string]any{
					"error":    "link_required",
					"provider": string(pname),
				})
				return
			}
			// New user with this email does not exist: do NOT auto-create or auto-assign an organisation.
			// Instruct the user to sign up (which creates an org) or obtain an invite.
			if strings.TrimSpace(cfg.Frontend.URL) != "" {
				base := strings.TrimRight(cfg.Frontend.URL, "/")
				dest := base + "/account/register?reason=signup_required&provider=" + url.QueryEscape(string(pname)) + "&email=" + url.QueryEscape(id.Email)
				http.Redirect(w, req, dest, http.StatusFound)
				return
			}
			writeJSON(w, http.StatusConflict, map[string]any{
				"error":    "signup_required",
				"message":  "No account exists for this email. Please sign up or ask for an invite.",
				"provider": string(pname),
				"email":    id.Email,
			})
			return
		}

		var org models.Org
		if existingLinked {
			// For already linked accounts, do NOT change org membership; pick an existing org.
			org, err = r.PickUserOrg(ctx, u.ID)
			if err != nil {
				http.Error(w, "no organisation", http.StatusForbidden)
				return
			}
		} else {
			// New account: resolve org and ensure membership.
			org, err = resolveOrgPostLogin(ctx, r, pname, id)
			if err != nil {
				http.Error(w, "org resolution failed: "+err.Error(), http.StatusUnauthorized)
				return
			}
			// Ensure membership (JIT) + optionally map IdP groups -> role
			role, err := r.EnsureMembership(ctx, org.ID, u.ID, models.RoleMember)
			if err != nil {
				http.Error(w, "membership failed: "+err.Error(), http.StatusInternalServerError)
				return
			}
			if len(id.Groups) > 0 {
				if upgraded, err := r.ApplyGroupRoleMappings(ctx, org.ID, string(pname), id.Groups); err == nil && upgraded != "" {
					_ = role // you can persist upgraded role if desired
					role = upgraded
				}
			}
		}

		// Create session
		SetSessionCookie(w, models.Session{
			UserID:    u.ID,
			ActiveOrg: org.ID,
			Provider:  string(pname),
			Expiry:    time.Now().Add(8 * time.Hour),
		})

		// Record successful provider login for last_login metrics
		if ip, ok := clientIP(req); ok {
			_ = r.RecordLoginSuccess(ctx, u.Email, ip)
		}

		// Redirect to frontend if configured; otherwise fallback to current host
		if strings.TrimSpace(cfg.Frontend.URL) != "" {
			base := strings.TrimRight(cfg.Frontend.URL, "/")
			path := cfg.Frontend.PostLoginPath
			if strings.TrimSpace(path) == "" {
				path = "/app/work-orders"
			}
			path = "/" + strings.TrimLeft(path, "/")
			http.Redirect(w, req, base+path, http.StatusFound)
			return
		}

		// Fallback uses same host and scheme inferred from headers
		scheme := req.Header.Get("X-Forwarded-Proto")
		if scheme == "" {
			scheme = "http"
		}
		host := req.Header.Get("X-Forwarded-Host")
		if host == "" {
			host = req.Host
		}
		http.Redirect(w, req, scheme+"://"+host+"/app/work-orders", http.StatusFound)
	}
}

func ProfileHandler(r repo.Repo) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// GET /auth/me
		sess := ReadSession(req)
		if sess == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		user, org, role, err := r.GetUserWithOrgAndRole(req.Context(), sess.UserID, sess.ActiveOrg)
		if err != nil {
			fmt.Printf("ProfileHandler: error fetching user/org/role: %v\n", err)
			if errors.Is(err, models.ErrUserNotFound) {
				http.Error(w, "user not found", http.StatusNotFound)
			} else if errors.Is(err, models.ErrOrgNotFound) {
				http.Error(w, "org not found", http.StatusNotFound)
			} else if errors.Is(err, models.ErrRoleNotFound) {
				http.Error(w, "role not found", http.StatusNotFound)
			}
			http.Error(w, "internal error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		// Fetch full user fields (avatar, country, phone)
		full, err := r.GetUserByID(req.Context(), user.ID)
		if err == nil {
			user = full
		}

		// Return only safe, self-profile fields
		resp := map[string]any{
			"email":      user.Email,
			"name":       user.Name,
			"avatar_url": user.AvatarURL,
			"country":    user.Country,
			"org":        org.Slug,
			"role":       role,
			"provider":   sess.Provider,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

// --- Identity extraction helpers ---

type identity struct {
	Email   string
	Name    string
	Subject string // stable provider user id (OIDC sub, GitHub ID)
	Tenant  string // Microsoft tid
	Groups  []string
	Picture string // avatar/profile picture URL if available
}

// FullProfileHandler returns extended information including linked providers,
// organisations and last login.
func FullProfileHandler(r repo.Repo) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		sess := ReadSession(req)
		if sess == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		// Base user and org list
		user, _, _, err := r.GetUserWithOrgAndRole(req.Context(), sess.UserID, sess.ActiveOrg)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		full, err := r.GetUserByID(req.Context(), user.ID)
		if err == nil {
			user = full
		}
		ids, _ := r.ListIdentitiesForUser(req.Context(), user.ID)
		orgs, _ := r.ListUserOrgs(req.Context(), user.ID)
		lastLogin, ok := r.GetLastSuccessfulLoginByUsername(req.Context(), strings.ToLower(user.Email))

		resp := map[string]any{
			"user": map[string]any{
				"email":      user.Email,
				"name":       user.Name,
				"avatar_url": user.AvatarURL,
				"phone":      user.Phone,
				"country":    user.Country,
			},
			"linked_providers": ids,
			"organisations":    orgs,
		}
		if ok {
			resp["last_login"] = lastLogin
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func extractIdentity(ctx context.Context, pname ProviderKind, p *Provider, tok *oauth2.Token, wantNonce string) (identity, error) {
	// OIDC providers (Microsoft/Google)
	if p.OIDCVerifier != nil {
		raw, _ := tok.Extra("id_token").(string)
		if raw == "" {
			return identity{}, errors.New("no id_token in response")
		}
		idt, err := p.OIDCVerifier.Verify(ctx, raw)
		if err != nil {
			return identity{}, err
		}

		switch pname {
		case ProviderMicrosoft:
			var c msClaims
			if err := idt.Claims(&c); err != nil {
				return identity{}, err
			}
			if wantNonce != "" && c.Nonce != wantNonce {
				return identity{}, errors.New("bad nonce")
			}
			email := firstNonEmpty(c.Email, c.PreferredUser)
			return identity{
				Email:   email,
				Name:    c.Name,
				Subject: c.Sub,
				Tenant:  c.TenantID,
				Groups:  c.Groups,
			}, nil

		default: // Google (and other OIDC)
			var c struct {
				Email         string `json:"email"`
				EmailVerified bool   `json:"email_verified"`
				Name          string `json:"name"`
				Sub           string `json:"sub"`
				Nonce         string `json:"nonce"`
				Picture       string `json:"picture"`
			}
			if err := idt.Claims(&c); err != nil {
				return identity{}, err
			}
			if wantNonce != "" && c.Nonce != wantNonce {
				return identity{}, errors.New("bad nonce")
			}
			return identity{
				Email:   c.Email,
				Name:    c.Name,
				Subject: c.Sub,
				Picture: c.Picture,
			}, nil
		}
	}

	// OAuth-only (GitHub)
	if pname == ProviderGitHub {
		email, name, id, avatar, err := fetchGitHubProfile(tok.AccessToken)
		if err != nil {
			return identity{}, err
		}
		return identity{
			Email:   email,
			Name:    name,
			Subject: id,
			Picture: avatar,
		}, nil
	}

	return identity{}, errors.New("unsupported provider")
}

// Microsoft claims for id_token
type msClaims struct {
	Email         string   `json:"email"`
	EmailVerified bool     `json:"email_verified"`
	Name          string   `json:"name"`
	PreferredUser string   `json:"preferred_username"`
	Sub           string   `json:"sub"`
	TenantID      string   `json:"tid"`
	Groups        []string `json:"groups,omitempty"`
	Nonce         string   `json:"nonce"`
}

// resolveOrgPostLogin picks an organisation after login.
// Strategy:
//   - If Microsoft and tid is present → find by tenant ID
//   - else: try email domain → slug
//   - fallback to a default slug "acme"
func resolveOrgPostLogin(ctx context.Context, r repo.Repo, pname ProviderKind, id identity) (models.Org, error) {
	// Microsoft tenant mapping
	if pname == ProviderMicrosoft && id.Tenant != "" {
		if org, err := r.FindOrgByTenantID(ctx, id.Tenant); err == nil {
			return org, nil
		}
	}
	// Email domain convention: "acme.com" -> slug "acme"
	if at := strings.LastIndex(id.Email, "@"); at > 0 && at < len(id.Email)-1 {
		domain := id.Email[at+1:]
		if dot := strings.Index(domain, "."); dot > 0 {
			slug := strings.ToLower(domain[:dot])
			if org, err := r.FindOrgBySlug(ctx, slug); err == nil {
				return org, nil
			}
		}
	}
	// Fallback
	return r.FindOrgBySlug(ctx, "acme")
}

// --- small utils ---

func setTempCookie(w http.ResponseWriter, name, val string, ttl time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    val,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(ttl),
	})
}

func readCookie(r *http.Request, name string) string {
	if c, err := r.Cookie(name); err == nil && c != nil {
		return c.Value
	}
	return ""
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

// fetchGitHubProfile fetches the basic user profile (and a best-effort email)
// using the provided access token. For production, also call /user/emails to
// pick the primary, verified email.
func fetchGitHubProfile(accessToken string) (email, name, id, avatar string, err error) {
	req, _ := http.NewRequest("GET", "https://api.github.com/user", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", "", "", fmt.Errorf("github user: status %d", resp.StatusCode)
	}
	var u struct {
		ID    int64  `json:"id"`
		Login string `json:"login"`
		Name  string `json:"name"`
		Email string `json:"email"` // often empty
	}
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return "", "", "", "", err
	}

	email = u.Email
	if email == "" && u.Login != "" {
		email = u.Login + "@users.noreply.github.com"
	}
	name = firstNonEmpty(u.Name, u.Login)
	id = fmt.Sprintf("%d", u.ID)
	// GitHub v3 user API also returns avatar_url; fetch if available via a second call or include in struct
	// For simplicity, we call /user again with Accept header; many responses include avatar_url
	// But to avoid another round-trip, assume avatar field exists; if not, leave empty.
	type gh2 struct {
		AvatarURL string `json:"avatar_url"`
	}
	// Quick refetch to get avatar_url without additional scopes
	if avatar == "" {
		req2, _ := http.NewRequest("GET", "https://api.github.com/user", nil)
		req2.Header.Set("Authorization", "Bearer "+accessToken)
		req2.Header.Set("Accept", "application/vnd.github+json")
		if resp2, err2 := http.DefaultClient.Do(req2); err2 == nil {
			defer resp2.Body.Close()
			var t gh2
			_ = json.NewDecoder(resp2.Body).Decode(&t)
			avatar = t.AvatarURL
		}
	}
	return
}
