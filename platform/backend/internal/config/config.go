// internal/config/config.go
package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DBHost     string `env:"DB_HOST"`
	DBPort     string `env:"DB_PORT"`
	DBUser     string `env:"DB_USER"`
	DBPassword string `env:"DB_PASSWORD"`
	DBName     string `env:"DB_NAME"`
	DBSSLMode  string `env:"DB_SSLMODE"`

	JWTSecret  string `env:"JWT_SECRET"`

	ServerPort string `env:"SERVER_PORT"` // new field for Echo
}

func Load() *Config {
	// Load .env file (ignored in Docker)
	_ = godotenv.Load()

	cfg := &Config{
		DBHost:     getEnv("DB_HOST", "postgres"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "obsuser"),
		DBPassword: getEnv("DB_PASSWORD", "pass1234"),
		DBName:     getEnv("DB_NAME", "obs_saas_dev"),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),

		JWTSecret:  getEnv("JWT_SECRET", "super-secret-change-me-in-prod-32-chars-min"),

		ServerPort: getEnv("SERVER_PORT", "8080"), // default to 8080
	}

	// Required fields validation
	required := []string{
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName,
		cfg.JWTSecret,
	}
	for _, v := range required {
		if v == "" {
			log.Fatal("Missing required configuration value")
		}
	}

	if len(cfg.JWTSecret) < 32 {
		log.Fatal("JWT_SECRET must be at least 32 characters")
	}

	log.Printf("[CONFIG] Loaded: DB=%s@%s:%s/%s | Port=%s",
		cfg.DBUser, cfg.DBHost, cfg.DBPort, cfg.DBName, cfg.ServerPort)

	return cfg
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value
	}
	return fallback
}