// internal/service/admin_auth_service.go
package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

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

type AdminAuthService struct {
	db         *gorm.DB
	redis      *redis.Client
	cfg        *config.Config
	adminRepo  *repository.AdminRepository
}

func NewAdminAuthService(db *gorm.DB, redisClient *redis.Client, cfg *config.Config, adminRepo *repository.AdminRepository) *AdminAuthService {
	return &AdminAuthService{
		db:        db,
		redis:     redisClient,
		cfg:       cfg,
		adminRepo: adminRepo,
	}
}

type adminRefreshSession struct {
	AdminID uint   `json:"admin_id"`
	Email   string `json:"email"`
	Role    string `json:"role"`
}

func (s *AdminAuthService) Signup(ctx context.Context, req dto.AdminSignupRequest) (accessToken string, refreshToken string, err error) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(req.Email))

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		adminRepo := s.adminRepo.WithDB(tx)

		if existing, err := adminRepo.FindByEmailCtx(ctx, normalizedEmail); err != nil {
			return err
		} else if existing != nil {
			return apierror.ErrCompanyEmailConflict
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}

		admin := &domain.Admin{
			Email:        normalizedEmail,
			PasswordHash: string(hash),
			FullName:     req.FullName,
			IsActive:     true,
		}
		return adminRepo.Create(admin)
	}); err != nil {
		return "", "", err
	}

	admin, err := s.adminRepo.WithDB(s.db).FindByEmailCtx(context.Background(), normalizedEmail)
	if err != nil || admin == nil {
		return "", "", apierror.ErrInternal
	}

	accessToken, refreshToken, err = s.issueTokens(admin)
	if err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

func (s *AdminAuthService) Login(ctx context.Context, email, password string) (accessToken string, refreshToken string, err error) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))

	if s.isLoginLocked(ctx, normalizedEmail) {
		return "", "", apierror.ErrLoginLocked
	}

	admin, err := s.adminRepo.WithDB(s.db).FindByEmailCtx(ctx, normalizedEmail)
	if err != nil || admin == nil {
		s.recordFailedLogin(ctx, normalizedEmail)
		return "", "", apierror.ErrInvalidCredentials
	}
	if !admin.IsActive {
		return "", "", apierror.ErrUserDeactivated
	}

	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(password)); err != nil {
		s.recordFailedLogin(ctx, normalizedEmail)
		return "", "", apierror.ErrInvalidCredentials
	}

	s.clearFailedLogins(ctx, normalizedEmail)
	accessToken, refreshToken, err = s.issueTokens(admin)
	if err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

func (s *AdminAuthService) Refresh(ctx context.Context, refreshToken string) (string, string, error) {
	if s.redis == nil || s.cfg == nil {
		return "", "", apierror.ErrAuthTemporarilyUnavailable
	}
	if refreshToken == "" {
		return "", "", apierror.ErrUnauthorized
	}

	oldHash, err := infrastructure.HashRefreshToken(refreshToken)
	if err != nil {
		return "", "", apierror.ErrUnauthorized
	}
	oldKey := "admin:refresh:" + oldHash

	rawNewRefresh, newHash, err := infrastructure.GenerateRefreshToken()
	if err != nil {
		return "", "", err
	}
	newKey := "admin:refresh:" + newHash

	ttlSeconds := int64(time.Duration(s.cfg.RefreshTokenTTLDays) * 24 * time.Hour / time.Second)
	if ttlSeconds <= 0 {
		return "", "", apierror.ErrInternal
	}

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

	var session adminRefreshSession
	if err := json.Unmarshal([]byte(valStr), &session); err != nil {
		return "", "", apierror.ErrUnauthorized
	}

	admin, err := s.adminRepo.WithDB(s.db).FindByIDCtx(ctx, session.AdminID)
	if err != nil || admin == nil {
		return "", "", apierror.ErrUnauthorized
	}
	if !admin.IsActive {
		return "", "", apierror.ErrUserDeactivated
	}

	newAccessToken, err := infrastructure.GenerateAdminToken(session.AdminID, session.Email, session.Role)
	if err != nil {
		return "", "", err
	}

	return newAccessToken, rawNewRefresh, nil
}

func (s *AdminAuthService) Logout(ctx context.Context, refreshToken string) error {
	if refreshToken == "" || s.redis == nil {
		return nil
	}
	hash, err := infrastructure.HashRefreshToken(refreshToken)
	if err != nil {
		return nil
	}
	return s.redis.Del(ctx, "admin:refresh:"+hash).Err()
}

func (s *AdminAuthService) ChangePassword(ctx context.Context, adminID uint, currentPassword, newPassword string) error {
	admin, err := s.adminRepo.WithDB(s.db).FindByIDCtx(ctx, adminID)
	if err != nil || admin == nil {
		return apierror.ErrUnauthorized
	}

	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(currentPassword)); err != nil {
		return apierror.ErrInvalidCredentials
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return apierror.ErrInternal
	}

	return s.adminRepo.UpdatePasswordCtx(ctx, adminID, string(newHash))
}

func (s *AdminAuthService) issueTokens(admin *domain.Admin) (accessToken string, refreshToken string, err error) {
	if s.redis == nil || s.cfg == nil {
		return "", "", apierror.ErrAuthTemporarilyUnavailable
	}

	accessToken, err = infrastructure.GenerateAdminToken(admin.ID, admin.Email, domain.AdminRoleSuperAdmin)
	if err != nil {
		return "", "", err
	}

	rawRefresh, refreshHash, err := infrastructure.GenerateRefreshToken()
	if err != nil {
		return "", "", err
	}

	session := adminRefreshSession{
		AdminID: admin.ID,
		Email:   admin.Email,
		Role:    domain.AdminRoleSuperAdmin,
	}
	data, err := json.Marshal(session)
	if err != nil {
		return "", "", err
	}

	ttl := time.Duration(s.cfg.RefreshTokenTTLDays) * 24 * time.Hour
	if err := s.redis.Set(context.Background(), "admin:refresh:"+refreshHash, data, ttl).Err(); err != nil {
		return "", "", apierror.ErrAuthTemporarilyUnavailable
	}

	return accessToken, rawRefresh, nil
}

func (s *AdminAuthService) isLoginLocked(ctx context.Context, email string) bool {
	if s.redis == nil || s.cfg == nil {
		return false
	}
	key := "auth:lock:admin:email:" + email
	exists, err := s.redis.Exists(ctx, key).Result()
	if err != nil {
		return false
	}
	return exists > 0
}

func (s *AdminAuthService) recordFailedLogin(ctx context.Context, email string) {
	if s.redis == nil || s.cfg == nil {
		return
	}
	failKey := "auth:fail:admin:email:" + email
	lockKey := "auth:lock:admin:email:" + email

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

func (s *AdminAuthService) clearFailedLogins(ctx context.Context, email string) {
	if s.redis == nil {
		return
	}
	_ = s.redis.Del(ctx, "auth:fail:admin:email:"+email, "auth:lock:admin:email:"+email).Err()
}

func (s *AdminAuthService) GetProfile(ctx context.Context, adminID uint) (*domain.Admin, error) {
	return s.adminRepo.WithDB(s.db).FindByIDCtx(ctx, adminID)
}
