// internal/infrastructure/redis.go
package infrastructure

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/config"
)

var Redis *redis.Client

func ConnectRedis(cfg *config.Config) {
	Redis = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", cfg.RedisHost, cfg.RedisPort),
		Password: cfg.RedisPassword,
		DB:       0,

		// Connection pool — mirrors the Postgres pool sizing
		PoolSize:     25,
		MinIdleConns: 5,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := Redis.Ping(ctx).Err(); err != nil {
		log.Fatalf("[REDIS] Failed to connect: %v", err)
	}

	log.Printf("[REDIS] Connected to %s:%s", cfg.RedisHost, cfg.RedisPort)
}

func PingRedis() bool {
	if Redis == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return Redis.Ping(ctx).Err() == nil
}