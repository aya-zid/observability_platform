// internal/config/config.go
package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	// ── App ───────────────────────────────────────────────────
	AppEnv string // "development" | "production"

	// ── Database ──────────────────────────────────────────────
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	// ── Auth ──────────────────────────────────────────────────
	JWTSecret             string
	RefreshTokenPepper    string
	JWTIssuer             string
	JWTAudience           string
	AccessTokenTTLMinutes int
	RefreshTokenTTLDays   int

	// ── Cookies (browser auth) ─────────────────────────────────
	CookieDomain   string
	CookieSecure   bool
	CookieSameSite string // "Lax" | "Strict" | "None"

	// ── CORS (dev frontend) ────────────────────────────────────
	CORSAllowOrigins []string

	// ── Auth Security ──────────────────────────────────────────
	AuthRateLimitWindowSeconds    int
	AuthLoginRateLimitPerWindow   int
	AuthSignupRateLimitPerWindow  int
	AuthRefreshRateLimitPerWindow int

	AuthMaxFailedLogins          int
	AuthFailedLoginWindowSeconds int
	AuthLockoutSeconds           int

	// ── Server ────────────────────────────────────────────────
	ServerPort string

	// ── Redis ─────────────────────────────────────────────────
	RedisHost     string
	RedisPort     string
	RedisPassword string // empty = no auth (fine for internal Docker network)

	// ── Observability infra (internal Docker network addresses) ──
	// These are NEVER exposed to agents — only used inside the proxy
	VMInsertHost string // e.g. "vminsert"
	VMInsertPort string // e.g. "8480"
	VMSelectHost string // e.g. "vmselect"
	VMSelectPort string // e.g. "8481"
	LokiHost     string // e.g. "loki"
	LokiPort     string // e.g. "3100"
	TempoHost    string // e.g. "tempo"
	TempoPort    string // e.g. "4318"
	GrafanaURL   string // e.g. "http://localhost:3000"

	// ── Agent installer script ─────────────────────────────────
	AgentInstallScriptPath string

	// ── Vulnerability scanning ─────────────────────────────────
	NVDAPIKey                    string
	VulnerabilityScanEnabled     bool
	VulnerabilityScanInterval    time.Duration
	VulnerabilitySeverityMinimum string
}

func Load() *Config {
	_ = godotenv.Load()

	cfg := &Config{
		// App
		AppEnv: getEnv("APP_ENV", "development"),

		// Database
		DBHost:     getEnv("DB_HOST", "postgres"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "obsuser"),
		DBPassword: getEnv("DB_PASSWORD", "pass1234"),
		DBName:     getEnv("DB_NAME", "obs_saas_dev"),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),

		// Auth
		JWTSecret:             getEnv("JWT_SECRET", "super-secret-change-me-in-prod-32-chars-min"),
		RefreshTokenPepper:    getEnv("REFRESH_TOKEN_PEPPER", ""),
		JWTIssuer:             getEnv("JWT_ISSUER", "obs-platform"),
		JWTAudience:           getEnv("JWT_AUDIENCE", "obs-api"),
		AccessTokenTTLMinutes: getEnvInt("ACCESS_TOKEN_TTL_MINUTES", 15),
		RefreshTokenTTLDays:   getEnvInt("REFRESH_TOKEN_TTL_DAYS", 7),

		// Cookies
		CookieDomain:   getEnv("COOKIE_DOMAIN", ""),
		CookieSecure:   getEnvBool("COOKIE_SECURE", false),
		CookieSameSite: normalizeSameSite(getEnv("COOKIE_SAMESITE", "Lax")),

		// CORS
		CORSAllowOrigins: parseCSV(getEnv("CORS_ALLOW_ORIGINS", "")),

		// Auth security
		AuthRateLimitWindowSeconds:    getEnvInt("AUTH_RATE_LIMIT_WINDOW_SECONDS", 60),
		AuthLoginRateLimitPerWindow:   getEnvInt("AUTH_LOGIN_RATE_LIMIT_PER_WINDOW", 20),
		AuthSignupRateLimitPerWindow:  getEnvInt("AUTH_SIGNUP_RATE_LIMIT_PER_WINDOW", 10),
		AuthRefreshRateLimitPerWindow: getEnvInt("AUTH_REFRESH_RATE_LIMIT_PER_WINDOW", 60),

		AuthMaxFailedLogins:          getEnvInt("AUTH_MAX_FAILED_LOGINS", 5),
		AuthFailedLoginWindowSeconds: getEnvInt("AUTH_FAILED_LOGIN_WINDOW_SECONDS", 900),
		AuthLockoutSeconds:           getEnvInt("AUTH_LOCKOUT_SECONDS", 900),

		// Server
		ServerPort: getEnv("SERVER_PORT", "8080"),

		// Redis
		RedisHost:     getEnv("REDIS_HOST", "redis"),
		RedisPort:     getEnv("REDIS_PORT", "6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),

		// Infra
		VMInsertHost: getEnv("VMINSERT_HOST", "vminsert"),
		VMInsertPort: getEnv("VMINSERT_PORT", "8480"),
		VMSelectHost: getEnv("VMSELECT_HOST", "vmselect"),
		VMSelectPort: getEnv("VMSELECT_PORT", "8481"),
		LokiHost:     getEnv("LOKI_HOST", "loki"),
		LokiPort:     getEnv("LOKI_PORT", "3100"),
		TempoHost:    getEnv("TEMPO_HOST", "tempo"),
		TempoPort:    getEnv("TEMPO_PORT", "4318"),
		GrafanaURL:   getEnv("GRAFANA_URL", "http://localhost:3000"),

		// Agent installer script path inside backend runtime
		AgentInstallScriptPath: getEnv("AGENT_INSTALL_SCRIPT_PATH", "/opt/agent/install.sh"),

		// Vulnerability scanning
		NVDAPIKey:                    getEnv("NVD_API_KEY", ""),
		VulnerabilityScanEnabled:     getEnvBool("VULNERABILITY_SCAN_ENABLED", true),
		VulnerabilityScanInterval:    time.Duration(getEnvInt("VULNERABILITY_SCAN_INTERVAL_HOURS", 2)) * time.Hour,
		VulnerabilitySeverityMinimum: strings.ToUpper(getEnv("VULNERABILITY_SEVERITY_MINIMUM", "MEDIUM")),
	}

	// Hard requirements
	if cfg.JWTSecret == "" {
		log.Fatal("[CONFIG] JWT_SECRET is required")
	}
	if len(cfg.JWTSecret) < 32 {
		log.Fatal("[CONFIG] JWT_SECRET must be at least 32 characters")
	}
	if cfg.RefreshTokenPepper == "" {
		log.Fatal("[CONFIG] REFRESH_TOKEN_PEPPER is required")
	}
	if len(cfg.RefreshTokenPepper) < 32 {
		log.Fatal("[CONFIG] REFRESH_TOKEN_PEPPER must be at least 32 characters")
	}
	if cfg.JWTIssuer == "" {
		log.Fatal("[CONFIG] JWT_ISSUER is required")
	}
	if cfg.JWTAudience == "" {
		log.Fatal("[CONFIG] JWT_AUDIENCE is required")
	}
	if cfg.AppEnv != "development" && cfg.AppEnv != "production" {
		log.Fatal("[CONFIG] APP_ENV must be one of: development, production")
	}
	if cfg.AccessTokenTTLMinutes <= 0 || cfg.AccessTokenTTLMinutes > 24*60 {
		log.Fatal("[CONFIG] ACCESS_TOKEN_TTL_MINUTES must be between 1 and 1440")
	}
	if cfg.RefreshTokenTTLDays <= 0 || cfg.RefreshTokenTTLDays > 365 {
		log.Fatal("[CONFIG] REFRESH_TOKEN_TTL_DAYS must be between 1 and 365")
	}
	switch cfg.CookieSameSite {
	case "Lax", "Strict", "None":
	default:
		log.Fatal("[CONFIG] COOKIE_SAMESITE must be one of: Lax, Strict, None")
	}
	if cfg.CookieSameSite == "None" && !cfg.CookieSecure {
		log.Fatal("[CONFIG] COOKIE_SECURE must be true when COOKIE_SAMESITE=None")
	}
	if len(cfg.CORSAllowOrigins) == 0 {
		if cfg.AppEnv == "production" {
			log.Fatal("[CONFIG] CORS_ALLOW_ORIGINS is required in production")
		}
		// Dev default (explicit, visible): avoids silent misconfig when you forget env vars locally.
		cfg.CORSAllowOrigins = []string{"http://localhost:5173"}
		log.Printf("[CONFIG] CORS_ALLOW_ORIGINS not set — defaulting to http://localhost:5173 (development)")
	}
	if cfg.AuthRateLimitWindowSeconds <= 0 || cfg.AuthRateLimitWindowSeconds > 3600 {
		log.Fatal("[CONFIG] AUTH_RATE_LIMIT_WINDOW_SECONDS must be between 1 and 3600")
	}
	if cfg.AuthLoginRateLimitPerWindow <= 0 {
		log.Fatal("[CONFIG] AUTH_LOGIN_RATE_LIMIT_PER_WINDOW must be > 0")
	}
	if cfg.AuthSignupRateLimitPerWindow <= 0 {
		log.Fatal("[CONFIG] AUTH_SIGNUP_RATE_LIMIT_PER_WINDOW must be > 0")
	}
	if cfg.AuthRefreshRateLimitPerWindow <= 0 {
		log.Fatal("[CONFIG] AUTH_REFRESH_RATE_LIMIT_PER_WINDOW must be > 0")
	}
	if cfg.AuthMaxFailedLogins <= 0 || cfg.AuthMaxFailedLogins > 50 {
		log.Fatal("[CONFIG] AUTH_MAX_FAILED_LOGINS must be between 1 and 50")
	}
	if cfg.AuthFailedLoginWindowSeconds <= 0 || cfg.AuthFailedLoginWindowSeconds > 24*3600 {
		log.Fatal("[CONFIG] AUTH_FAILED_LOGIN_WINDOW_SECONDS must be between 1 and 86400")
	}
	if cfg.AuthLockoutSeconds <= 0 || cfg.AuthLockoutSeconds > 24*3600 {
		log.Fatal("[CONFIG] AUTH_LOCKOUT_SECONDS must be between 1 and 86400")
	}
	if cfg.VulnerabilityScanInterval < time.Hour {
		log.Fatal("[CONFIG] VULNERABILITY_SCAN_INTERVAL_HOURS must be at least 1")
	}
	switch cfg.VulnerabilitySeverityMinimum {
	case "MEDIUM", "HIGH", "CRITICAL":
	default:
		log.Fatal("[CONFIG] VULNERABILITY_SEVERITY_MINIMUM must be one of: MEDIUM, HIGH, CRITICAL")
	}

	log.Printf("[CONFIG] DB=%s@%s:%s/%s | Redis=%s:%s | Port=%s",
		cfg.DBUser, cfg.DBHost, cfg.DBPort, cfg.DBName,
		cfg.RedisHost, cfg.RedisPort, cfg.ServerPort)

	return cfg
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		i, err := strconv.Atoi(v)
		if err == nil {
			return i
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		switch strings.ToLower(v) {
		case "1", "true", "t", "yes", "y", "on":
			return true
		case "0", "false", "f", "no", "n", "off":
			return false
		}
	}
	return fallback
}

func normalizeSameSite(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "lax":
		return "Lax"
	case "strict":
		return "Strict"
	case "none":
		return "None"
	default:
		return v
	}
}

func parseCSV(v string) []string {
	var out []string
	for _, part := range strings.Split(v, ",") {
		p := strings.TrimSpace(part)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
