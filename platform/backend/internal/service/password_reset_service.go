// internal/service/password_reset_service.go
package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/apierror"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
)

type PasswordResetService struct {
	db             *gorm.DB
	userRepo       *repository.UserRepository
	resetTokenRepo *repository.PasswordResetRepository
	emailSender    *EmailSender
	platformURL    string
	tokenTTL       time.Duration
}

func NewPasswordResetService(db *gorm.DB, userRepo *repository.UserRepository, resetTokenRepo *repository.PasswordResetRepository, emailSender *EmailSender, platformURL string, tokenTTL time.Duration) *PasswordResetService {
	return &PasswordResetService{
		db:             db,
		userRepo:       userRepo,
		resetTokenRepo: resetTokenRepo,
		emailSender:    emailSender,
		platformURL:    strings.TrimRight(platformURL, "/"),
		tokenTTL:       tokenTTL,
	}
}

func generateResetToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(b), nil
}

// RequestPasswordReset issues a reset token and emails a reset link.
// It always returns nil for unknown emails to avoid leaking which accounts exist.
func (s *PasswordResetService) RequestPasswordReset(ctx context.Context, email string) error {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))

	user, err := s.userRepo.WithDB(s.db).FindByEmailCtx(ctx, normalizedEmail)
	if err != nil {
		return err
	}
	if user == nil {
		log.Printf("[password_reset] reset requested for unknown email %s (ignored)", normalizedEmail)
		return nil
	}

	token, err := generateResetToken()
	if err != nil {
		return err
	}

	reset := &domain.PasswordResetToken{
		UserID:    user.ID,
		Token:     token,
		ExpiresAt: time.Now().Add(s.tokenTTL),
	}

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		resetTokenRepo := s.resetTokenRepo.WithDB(tx)
		if err := resetTokenRepo.InvalidateForUserCtx(ctx, user.ID); err != nil {
			return err
		}
		return resetTokenRepo.Create(reset)
	}); err != nil {
		return err
	}

	resetURL := fmt.Sprintf("%s/reset-password/%s", s.platformURL, token)
	subject := "Reset your MediaNet Observability password"
	body := s.emailSender.PasswordResetHTML(resetURL)
	if err := s.emailSender.Send(normalizedEmail, subject, body); err != nil {
		return fmt.Errorf("send reset email: %w", err)
	}
	log.Printf("[password_reset] reset email sent to %s", normalizedEmail)
	return nil
}

// ValidateResetToken returns the associated user if the token is valid, unused and not expired.
func (s *PasswordResetService) ValidateResetToken(ctx context.Context, token string) (*domain.User, error) {
	reset, err := s.findValidToken(ctx, token)
	if err != nil {
		return nil, err
	}
	user, err := s.userRepo.WithDB(s.db).FindByIDCtx(ctx, reset.UserID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, apierror.ErrInvalidResetToken
	}
	return user, nil
}

// ResetPassword validates the token and applies the new password hash.
func (s *PasswordResetService) ResetPassword(ctx context.Context, token, password string) error {
	reset, err := s.findValidToken(ctx, token)
	if err != nil {
		return err
	}

	user, err := s.userRepo.WithDB(s.db).FindByIDCtx(ctx, reset.UserID)
	if err != nil {
		return err
	}
	if user == nil {
		return apierror.ErrInvalidResetToken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		userRepo := s.userRepo.WithDB(tx)
		resetTokenRepo := s.resetTokenRepo.WithDB(tx)
		if err := userRepo.UpdatePasswordCtx(ctx, user.ID, string(hash)); err != nil {
			return err
		}
		return resetTokenRepo.InvalidateForUserCtx(ctx, user.ID)
	})
}

func (s *PasswordResetService) findValidToken(ctx context.Context, token string) (*domain.PasswordResetToken, error) {
	reset, err := s.resetTokenRepo.WithDB(s.db).FindByTokenCtx(ctx, token)
	if err != nil {
		return nil, err
	}
	if reset == nil || reset.Used || time.Now().After(reset.ExpiresAt) {
		return nil, apierror.ErrInvalidResetToken
	}
	return reset, nil
}
