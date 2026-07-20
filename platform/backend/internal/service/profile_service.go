package service

import (
	"context"
	"errors"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/apierror"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
)

type ProfileService struct {
	db       *gorm.DB
	userRepo *repository.UserRepository
}

func NewProfileService(db *gorm.DB, userRepo *repository.UserRepository) *ProfileService {
	return &ProfileService{db: db, userRepo: userRepo}
}

func (s *ProfileService) UpdateProfile(ctx context.Context, userID uint, fullName string) error {
	return s.db.WithContext(ctx).Table("users").Where("id = ?", userID).Update("full_name", fullName).Error
}

func (s *ProfileService) ChangePassword(ctx context.Context, userID uint, currentPassword string, newPassword string) error {
	user, err := s.userRepo.FindByIDCtx(ctx, userID)
	if err != nil {
		return err
	}
	if user == nil {
		return errors.New("user not found")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)); err != nil {
		return apierror.ErrInvalidCredentials
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	return s.userRepo.UpdatePasswordCtx(ctx, userID, string(hash))
}
