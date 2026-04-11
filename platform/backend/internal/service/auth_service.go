// internal/service/auth_service.go
package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/apierror"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/config"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/dto"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/infrastructure"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
)

type AuthService struct {
	db         *gorm.DB
	redis      *redis.Client
	cfg        *config.Config
	tenantRepo *repository.TenantRepository
	userRepo   *repository.UserRepository
}

func NewAuthService(db *gorm.DB, redisClient *redis.Client, cfg *config.Config, tenantRepo *repository.TenantRepository, userRepo *repository.UserRepository) *AuthService {
	return &AuthService{
		db:         db,
		redis:      redisClient,
		cfg:        cfg,
		tenantRepo: tenantRepo,
		userRepo:   userRepo,
	}
}

type refreshSession struct {
	UserID   uint   `json:"user_id"`
	TenantID uint   `json:"tenant_id"`
	Email    string `json:"email"`
	Role     string `json:"role"`
}

func (s *AuthService) Signup(ctx context.Context, req dto.SignupRequest) (accessToken string, refreshToken string, err error) {
	var createdUser *domain.User
	normalizedBusinessEmail := strings.ToLower(strings.TrimSpace(req.BusinessEmail))

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		tenantRepo := s.tenantRepo.WithDB(tx)
		userRepo := s.userRepo.WithDB(tx)

		// Check uniqueness (best-effort UX). DB unique constraints remain the source of truth.
		if t, err := tenantRepo.FindByEmailCtx(ctx, normalizedBusinessEmail); err != nil {
			return err
		} else if t != nil {
			return apierror.ErrCompanyEmailConflict
		}
		if u, err := userRepo.FindByEmailCtx(ctx, normalizedBusinessEmail); err != nil {
			return err
		} else if u != nil {
			return apierror.ErrUserEmailConflict
		}

		// Hash password
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}

		// Generate API key (stored in clear text by request)
		rawAPIKey, err := infrastructure.GenerateTenantAPIKey()
		if err != nil {
			return err
		}

		// Create tenant
		createdTenant := &domain.Tenant{
			UUID:               uuid.New().String(),
			CompanyName:        req.CompanyName,
			ContactEmail:       normalizedBusinessEmail,
			ContactPhone:       req.ContactPhone,
			IsActive:           true,
			SubscriptionStatus: domain.SubscriptionStatusActive,
			APIKeyEnabled:      true,
			APIKey:             rawAPIKey,
		}
		if err := tenantRepo.Create(createdTenant); err != nil {
			return err
		}

		// Create admin user
		createdUser = &domain.User{
			TenantID:     createdTenant.ID,
			FullName:     req.FullName,
			Email:        normalizedBusinessEmail,
			PasswordHash: string(hash),
			Role:         "admin",
		}
		if err := userRepo.Create(createdUser); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return "", "", err
	}

	accessToken, refreshToken, err = s.issueTokens(ctx, createdUser)
	if err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (accessToken string, refreshToken string, err error) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))

	if s.isLoginLocked(ctx, normalizedEmail) {
		return "", "", apierror.ErrLoginLocked
	}

	user, err := s.userRepo.WithDB(s.db).FindByEmailCtx(ctx, normalizedEmail)
	if err != nil || user == nil {
		s.recordFailedLogin(ctx, normalizedEmail)
		return "", "", apierror.ErrInvalidCredentials
	}
	tenant, err := s.tenantRepo.WithDB(s.db).FindByIDCtx(ctx, user.TenantID)
	if err != nil || tenant == nil {
		return "", "", apierror.ErrUnauthorized
	}
	if !tenant.IsActive {
		return "", "", apierror.ErrUnauthorized
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		s.recordFailedLogin(ctx, normalizedEmail)
		return "", "", apierror.ErrInvalidCredentials
	}

	accessToken, refreshToken, err = s.issueTokens(ctx, user)
	if err != nil {
		return "", "", err
	}
	s.clearFailedLogins(ctx, normalizedEmail)
	return accessToken, refreshToken, nil
}

func (s *AuthService) isLoginLocked(ctx context.Context, email string) bool {
	if s.redis == nil || s.cfg == nil {
		return false
	}
	key := "auth:lock:email:" + email
	exists, err := s.redis.Exists(ctx, key).Result()
	if err != nil {
		return false
	}
	return exists > 0
}

func (s *AuthService) recordFailedLogin(ctx context.Context, email string) {
	if s.redis == nil || s.cfg == nil {
		return
	}
	failKey := "auth:fail:email:" + email
	lockKey := "auth:lock:email:" + email

	n, err := s.redis.Incr(ctx, failKey).Result()
	if err != nil {
		return
	}
	if n == 1 {
		_ = s.redis.Expire(ctx, failKey, time.Duration(s.cfg.AuthFailedLoginWindowSeconds)*time.Second).Err()
	}

	if int(n) >= s.cfg.AuthMaxFailedLogins {
		_ = s.redis.Set(ctx, lockKey, "1", time.Duration(s.cfg.AuthLockoutSeconds)*time.Second).Err()
	}
}

func (s *AuthService) clearFailedLogins(ctx context.Context, email string) {
	if s.redis == nil {
		return
	}
	_ = s.redis.Del(ctx, "auth:fail:email:"+email, "auth:lock:email:"+email).Err()
}

func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (string, string, error) {
	if s.redis == nil {
		return "", "", apierror.ErrAuthTemporarilyUnavailable
	}
	if s.cfg == nil {
		return "", "", apierror.ErrInternal
	}
	if refreshToken == "" {
		return "", "", apierror.ErrUnauthorized
	}

	oldHash, err := infrastructure.HashRefreshToken(refreshToken)
	if err != nil {
		return "", "", apierror.ErrUnauthorized
	}
	oldKey := "refresh:" + oldHash

	rawNewRefresh, newHash, err := infrastructure.GenerateRefreshToken()
	if err != nil {
		return "", "", err
	}
	newKey := "refresh:" + newHash

	ttlSeconds := int64(time.Duration(s.cfg.RefreshTokenTTLDays) * 24 * time.Hour / time.Second)
	if ttlSeconds <= 0 {
		return "", "", apierror.ErrInternal
	}

	// Atomically: read old session -> delete old -> write new with same session payload.
	// If anything fails, nothing is partially applied.
	script := `
local oldKey = KEYS[1]
local newKey = KEYS[2]
local ttl = tonumber(ARGV[1])
local val = redis.call("GET", oldKey)
if not val then
  return nil
end
redis.call("DEL", oldKey)
redis.call("SET", newKey, val, "EX", ttl)
return val
	`
	val, err := s.redis.Eval(ctx, script, []string{oldKey, newKey}, ttlSeconds).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", "", apierror.ErrUnauthorized
		}
		return "", "", apierror.ErrAuthTemporarilyUnavailable
	}
	if val == nil {
		return "", "", apierror.ErrUnauthorized
	}
	valStr, ok := val.(string)
	if !ok || valStr == "" {
		return "", "", apierror.ErrUnauthorized
	}

	var session refreshSession
	if err := json.Unmarshal([]byte(valStr), &session); err != nil {
		return "", "", apierror.ErrUnauthorized
	}
	tenant, err := s.tenantRepo.WithDB(s.db).FindByIDCtx(ctx, session.TenantID)
	if err != nil || tenant == nil {
		return "", "", apierror.ErrUnauthorized
	}
	if !tenant.IsActive {
		return "", "", apierror.ErrUnauthorized
	}

	newAccessToken, err := infrastructure.GenerateToken(session.UserID, session.TenantID, session.Email, session.Role)
	if err != nil {
		return "", "", err
	}

	return newAccessToken, rawNewRefresh, nil
}

func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return nil
	}
	hash, err := infrastructure.HashRefreshToken(refreshToken)
	if err != nil {
		return nil
	}
	return s.redis.Del(ctx, "refresh:"+hash).Err()
}

func (s *AuthService) issueTokens(ctx context.Context, user *domain.User) (accessToken string, refreshToken string, err error) {
	if s.redis == nil {
		return "", "", apierror.ErrAuthTemporarilyUnavailable
	}
	if s.cfg == nil {
		return "", "", apierror.ErrInternal
	}

	accessToken, err = infrastructure.GenerateToken(user.ID, user.TenantID, user.Email, user.Role)
	if err != nil {
		return "", "", err
	}

	rawRefresh, refreshHash, err := infrastructure.GenerateRefreshToken()
	if err != nil {
		return "", "", err
	}

	session := refreshSession{
		UserID:   user.ID,
		TenantID: user.TenantID,
		Email:    user.Email,
		Role:     user.Role,
	}
	data, err := json.Marshal(session)
	if err != nil {
		return "", "", err
	}

	ttl := time.Duration(s.cfg.RefreshTokenTTLDays) * 24 * time.Hour
	if err := s.redis.Set(ctx, "refresh:"+refreshHash, data, ttl).Err(); err != nil {
		return "", "", apierror.ErrAuthTemporarilyUnavailable
	}

	return accessToken, rawRefresh, nil
}
