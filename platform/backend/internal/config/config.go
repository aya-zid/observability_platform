// internal/config/config.go
package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	// ── Database ──────────────────────────────────────────────
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	// ── Auth ──────────────────────────────────────────────────
	JWTSecret string

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
}

func Load() *Config {
	_ = godotenv.Load()

	cfg := &Config{
		// Database
		DBHost:     getEnv("DB_HOST", "postgres"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "obsuser"),
		DBPassword: getEnv("DB_PASSWORD", "pass1234"),
		DBName:     getEnv("DB_NAME", "obs_saas_dev"),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),

		// Auth
		JWTSecret: getEnv("JWT_SECRET", "super-secret-change-me-in-prod-32-chars-min"),

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
	}

	// Hard requirements
	if cfg.JWTSecret == "" {
		log.Fatal("[CONFIG] JWT_SECRET is required")
	}
	if len(cfg.JWTSecret) < 32 {
		log.Fatal("[CONFIG] JWT_SECRET must be at least 32 characters")
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