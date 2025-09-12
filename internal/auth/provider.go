// internal/auth/provider.go

package auth

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"

	"yourapp/internal/config" // Update with your actual module name
)

type ProviderKind string

const (
	ProviderMicrosoft ProviderKind = "microsoft"
	ProviderGoogle    ProviderKind = "google"
	ProviderGitHub    ProviderKind = "github"
)

type Provider struct {
	OAuth2Config *oauth2.Config
	OIDCVerifier *oidc.IDTokenVerifier
	Issuer       string
}

// SetupProviders initializes all enabled providers and returns them.
func SetupProviders(cfg config.Config) map[ProviderKind]*Provider {
	providers := map[ProviderKind]*Provider{}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Override the global HTTP client used by go-oidc
	original := http.DefaultClient
	http.DefaultClient = NewCachingHTTPClient(cfg.Auth.OIDCCacheDir)
	defer func() { http.DefaultClient = original }()

	type result struct {
		kind     ProviderKind
		provider *Provider
		err      error
	}

	ch := make(chan result, 2) // At most 2 OIDC providers concurrently

	// Microsoft OIDC
	if cfg.Microsoft.ClientID != "" {
		go func() {
			tenant := cfg.Microsoft.TenantID
			if tenant == "" {
				tenant = "organizations"
			}
			issuer := "https://login.microsoftonline.com/" + tenant + "/v2.0"

			oidcProv, err := oidc.NewProvider(ctx, issuer)
			if err != nil {
				ch <- result{kind: ProviderMicrosoft, err: err}
				return
			}

			conf := &oauth2.Config{
				ClientID:     cfg.Microsoft.ClientID,
				ClientSecret: cfg.Microsoft.ClientSecret,
				RedirectURL:  strings.TrimRight(cfg.BaseURL, "/") + "/auth/microsoft/callback",
				Endpoint:     oidcProv.Endpoint(),
				Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
			}

			ch <- result{
				kind: ProviderMicrosoft,
				provider: &Provider{
					OAuth2Config: conf,
					OIDCVerifier: oidcProv.Verifier(&oidc.Config{ClientID: conf.ClientID}),
					Issuer:       issuer,
				},
			}
		}()
	}

	// Google OIDC
	if cfg.Google.ClientID != "" {
		go func() {
			issuer := "https://accounts.google.com"

			oidcProv, err := oidc.NewProvider(ctx, issuer)
			if err != nil {
				ch <- result{kind: ProviderGoogle, err: err}
				return
			}

			conf := &oauth2.Config{
				ClientID:     cfg.Google.ClientID,
				ClientSecret: cfg.Google.ClientSecret,
				RedirectURL:  strings.TrimRight(cfg.BaseURL, "/") + "/auth/google/callback",
				Endpoint:     oidcProv.Endpoint(),
				Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
			}

			ch <- result{
				kind: ProviderGoogle,
				provider: &Provider{
					OAuth2Config: conf,
					OIDCVerifier: oidcProv.Verifier(&oidc.Config{ClientID: conf.ClientID}),
					Issuer:       issuer,
				},
			}
		}()
	}

	// Collect async results
	count := 0
	if cfg.Microsoft.ClientID != "" {
		count++
	}
	if cfg.Google.ClientID != "" {
		count++
	}

	for i := 0; i < count; i++ {
		res := <-ch
		if res.err != nil {
			panic(res.err) // You can replace this with proper error handling
		}
		providers[res.kind] = res.provider
	}

	// GitHub (OAuth2 only, no OIDC)
	if cfg.Github.ClientID != "" {
		conf := &oauth2.Config{
			ClientID:     cfg.Github.ClientID,
			ClientSecret: cfg.Github.ClientSecret,
			RedirectURL:  strings.TrimRight(cfg.BaseURL, "/") + "/auth/github/callback",
			Endpoint:     github.Endpoint,
			Scopes:       []string{"read:user", "user:email"},
		}
		providers[ProviderGitHub] = &Provider{OAuth2Config: conf}
	}

	go func() {
		refreshInterval := cfg.Auth.OIDCRefreshInterval
		if refreshInterval == 0 {
			refreshInterval = 6 * time.Hour // fallback default
		}
		//refreshInterval = 30 * time.Second // for testing
		log.Printf("[OIDC Cache] Background refresh started (every %s)", refreshInterval)

		transport := NewCachingTransport(cfg.Auth.OIDCCacheDir)
		ticker := time.NewTicker(refreshInterval)
		defer ticker.Stop()

		for range ticker.C {
			// Refresh known OIDC issuers
			for _, issuer := range []string{
				"https://login.microsoftonline.com/" + getTenant(cfg.Microsoft.TenantID) + "/v2.0/.well-known/openid-configuration",
				"https://accounts.google.com/.well-known/openid-configuration",
			} {
				if err := transport.ForceRefresh(issuer); err != nil {
					log.Printf("[OIDC Cache] REFRESH ERROR: %s", err)
				}
			}
		}
	}()

	return providers
}

func getTenant(id string) string {
	if id == "" {
		return "organizations"
	}
	return id
}
