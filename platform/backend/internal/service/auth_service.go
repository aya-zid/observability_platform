// internal/service/auth_service.go
package service

import (
	"errors"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/dto"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/infrastructure"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
)

type AuthService struct {
	tenantRepo *repository.TenantRepository
	userRepo   *repository.UserRepository
}

func NewAuthService(tenantRepo *repository.TenantRepository, userRepo *repository.UserRepository) *AuthService {
	return &AuthService{
		tenantRepo: tenantRepo,
		userRepo:   userRepo,
	}
}

func (s *AuthService) Signup(req dto.SignupRequest) (*dto.AuthResponse, error) {
	// Check uniqueness
	if t, _ := s.tenantRepo.FindByEmail(req.ContactEmail); t != nil {
		return nil, errors.New("company email already registered")
	}
	if u, _ := s.userRepo.FindByEmail(req.Email); u != nil {
		return nil, errors.New("user email already registered")
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	// Create tenant
	tenant := &domain.Tenant{
		UUID:         uuid.New().String(),
		CompanyName:  req.CompanyName,
		ContactEmail: req.ContactEmail,
		ContactPhone: req.ContactPhone,
		APIKey:       "obs_" + uuid.New().String()[:32], // placeholder - improve later
	}
	if err := s.tenantRepo.Create(tenant); err != nil {
		return nil, err
	}

	// Create admin user
	user := &domain.User{
		TenantID:     tenant.ID,
		FullName:     req.FullName,
		Email:        req.Email,
		PasswordHash: string(hash),
		Role:         "admin",
	}
	if err := s.userRepo.Create(user); err != nil {
		return nil, err
	}

	// Generate JWT
	token, err := infrastructure.GenerateToken(user.ID, tenant.ID, user.Email, user.Role)
	if err != nil {
		return nil, err
	}

	return &dto.AuthResponse{Token: token}, nil
}

func (s *AuthService) Login(email, password string) (*dto.AuthResponse, error) {
	user, err := s.userRepo.FindByEmail(email)
	if err != nil || user == nil {
		return nil, errors.New("invalid email or password")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, errors.New("invalid email or password")
	}

	token, err := infrastructure.GenerateToken(user.ID, user.TenantID, user.Email, user.Role)
	if err != nil {
		return nil, err
	}

	return &dto.AuthResponse{Token: token}, nil
}