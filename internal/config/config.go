// internal/config/config.go
package config

import (
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	BaseURL  string `mapstructure:"base_url"`
	Frontend struct {
		URL           string `mapstructure:"url"`
		PostLoginPath string `mapstructure:"post_login_path"`
		APIRoute      string `mapstructure:"api_route"`
	} `mapstructure:"frontend"`
	Database struct {
		URL string `mapstructure:"url"`
	} `mapstructure:"database"`
	Logging struct {
		Level  string `mapstructure:"level"`
		Format string `mapstructure:"format"`
	} `mapstructure:"logging"`
	Security struct {
		RequestID struct {
			TrustHeader bool `mapstructure:"trust_header"`
		} `mapstructure:"request_id"`
		Session struct {
			SweeperInterval time.Duration `mapstructure:"sweeper_interval"`
			CookieSecure    bool          `mapstructure:"cookie_secure"`
			SameSite        string        `mapstructure:"same_site"`
		} `mapstructure:"session"`
		MFA struct {
			LocalRequired bool `mapstructure:"local_required"`
		} `mapstructure:"mfa"`
		RateLimit struct {
			Enabled           bool          `mapstructure:"enabled"`
			RequestsPerMinute int           `mapstructure:"rpm"`
			Burst             int           `mapstructure:"burst"`
			TTL               time.Duration `mapstructure:"ttl"`
		} `mapstructure:"rate_limit"`
		Denylist struct {
			Enabled bool `mapstructure:"enabled"`
		} `mapstructure:"denylist"`
	} `mapstructure:"security"`
	Microsoft struct {
		ClientID     string `mapstructure:"client_id"`
		ClientSecret string `mapstructure:"client_secret"`
		TenantID     string `mapstructure:"tenant_id"`
	} `mapstructure:"microsoft"`
	Google struct {
		ClientID     string `mapstructure:"client_id"`
		ClientSecret string `mapstructure:"client_secret"`
	} `mapstructure:"google"`
	Github struct {
		ClientID     string `mapstructure:"client_id"`
		ClientSecret string `mapstructure:"client_secret"`
	} `mapstructure:"github"`
	Auth struct {
		OIDCCacheDir        string        `yaml:"oidcCacheDir"`
		OIDCRefreshInterval time.Duration `yaml:"oidcRefreshInterval"` // ← parseable duration
	} `yaml:"auth"`
}

func Load() Config {
	viper.SetDefault("microsoft.tenant_id", "organizations")
	// Frontend defaults
	viper.SetDefault("frontend.post_login_path", "/app/work-orders")
	viper.SetDefault("frontend.api_route", "") //or /api/backend
	// Sensible logging defaults
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "text")
	// Security defaults
	viper.SetDefault("security.request_id.trust_header", false)
	viper.SetDefault("security.session.sweeper_interval", "5m")
	viper.SetDefault("security.session.cookie_secure", false)
	viper.SetDefault("security.session.same_site", "lax")
	viper.SetDefault("security.mfa.local_required", false)
	viper.SetDefault("security.rate_limit.enabled", true)
	viper.SetDefault("security.rate_limit.rpm", 120)
	viper.SetDefault("security.rate_limit.burst", 60)
	viper.SetDefault("security.rate_limit.ttl", "30m")
	viper.SetDefault("security.denylist.enabled", true)

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("..")
	_ = viper.ReadInConfig()

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// explicit bindings
	_ = viper.BindEnv("base_url", "BASE_URL")
	_ = viper.BindEnv("frontend.url", "FRONTEND_URL")
	_ = viper.BindEnv("frontend.post_login_path", "FRONTEND_POST_LOGIN_PATH")
	_ = viper.BindEnv("frontend.api_route", "FRONTEND_API_ROUTE")
	_ = viper.BindEnv("database.url", "DATABASE_URL")
	_ = viper.BindEnv("logging.level", "LOG_LEVEL")
	_ = viper.BindEnv("logging.format", "LOG_FORMAT")
	_ = viper.BindEnv("security.request_id.trust_header", "REQUEST_ID_TRUST_HEADER")
	_ = viper.BindEnv("security.session.sweeper_interval", "SESSION_SWEEPER_INTERVAL")
	_ = viper.BindEnv("security.session.cookie_secure", "SESSION_COOKIE_SECURE")
	_ = viper.BindEnv("security.session.same_site", "SESSION_SAME_SITE")
	_ = viper.BindEnv("security.mfa.local_required", "MFA_LOCAL_REQUIRED")
	_ = viper.BindEnv("security.rate_limit.enabled", "RATE_LIMIT_ENABLED")
	_ = viper.BindEnv("security.rate_limit.rpm", "RATE_LIMIT_RPM")
	_ = viper.BindEnv("security.rate_limit.burst", "RATE_LIMIT_BURST")
	_ = viper.BindEnv("security.rate_limit.ttl", "RATE_LIMIT_TTL")
	_ = viper.BindEnv("security.denylist.enabled", "DENYLIST_ENABLED")
	_ = viper.BindEnv("microsoft.client_id", "MICROSOFT_CLIENT_ID")
	_ = viper.BindEnv("microsoft.client_secret", "MICROSOFT_CLIENT_SECRET")
	_ = viper.BindEnv("microsoft.tenant_id", "MICROSOFT_TENANT_ID")
	_ = viper.BindEnv("google.client_id", "GOOGLE_CLIENT_ID")
	_ = viper.BindEnv("google.client_secret", "GOOGLE_CLIENT_SECRET")
	_ = viper.BindEnv("github.client_id", "GITHUB_CLIENT_ID")
	_ = viper.BindEnv("github.client_secret", "GITHUB_CLIENT_SECRET")

	var c Config
	if err := viper.Unmarshal(&c); err != nil {
		panic("config error: " + err.Error())
	}
	// Normalize frontend API route: ensure leading '/' and no trailing '/'
	if strings.TrimSpace(c.Frontend.APIRoute) == "" {
		c.Frontend.APIRoute = ""
	} else {
		c.Frontend.APIRoute = "/" + strings.Trim(strings.TrimSpace(c.Frontend.APIRoute), "/")
	}
	if c.Frontend.APIRoute == "/" {
		c.Frontend.APIRoute = ""
	}
	if c.BaseURL == "" {
		panic("config error: base_url/BASE_URL required")
	}
	return c
}
