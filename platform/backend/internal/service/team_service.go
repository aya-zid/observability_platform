package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/apierror"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/dto"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
)

type TeamService struct {
	db               *gorm.DB
	userRepo         *repository.UserRepository
	invitationRepo   *repository.InvitationRepository
	tenantRepo       *repository.TenantRepository
	emailSender      *EmailSender
	platformURL      string
}

func NewTeamService(db *gorm.DB, userRepo *repository.UserRepository, invitationRepo *repository.InvitationRepository, tenantRepo *repository.TenantRepository, emailSender *EmailSender, platformURL string) *TeamService {
	return &TeamService{
		db:               db,
		userRepo:         userRepo,
		invitationRepo:   invitationRepo,
		tenantRepo:       tenantRepo,
		emailSender:      emailSender,
		platformURL:      strings.TrimRight(platformURL, "/"),
	}
}

func generateInviteToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(b), nil
}

func (s *TeamService) InviteTeammate(ctx context.Context, tenantID uint, inviterUserID uint, email string, role string) (*domain.Invitation, error) {
	if role != "admin" && role != "member" {
		return nil, apierror.ErrValidationFailed
	}

	existingUser, err := s.userRepo.FindByEmailCtx(ctx, email)
	if err != nil {
		return nil, err
	}
	if existingUser != nil && existingUser.TenantID == tenantID {
		return nil, errors.New("user already belongs to this tenant")
	}

	pending, err := s.invitationRepo.FindPendingByEmailAndTenantCtx(ctx, email, tenantID)
	if err != nil {
		return nil, err
	}
	if pending != nil {
		return nil, errors.New("pending invitation already exists for this email")
	}

	token, err := generateInviteToken()
	if err != nil {
		return nil, err
	}

	inv := &domain.Invitation{
		TenantID:  tenantID,
		Email:     email,
		Role:      role,
		Token:     token,
		InvitedBy: inviterUserID,
		Status:    "pending",
		ExpiresAt: time.Now().Add(72 * time.Hour),
	}
	if err := s.invitationRepo.Create(inv); err != nil {
		return nil, err
	}

	// Load tenant for company name
	tenant, err := s.tenantRepo.FindByIDCtx(ctx, tenantID)
	if err == nil && tenant != nil {
		inviter, _ := s.userRepo.FindByIDCtx(ctx, inviterUserID)
		inviterName := "A team admin"
		if inviter != nil && inviter.FullName != "" {
			inviterName = inviter.FullName
		}
		companyName := tenant.CompanyName
		if companyName == "" {
			companyName = "the team"
		}
		inviteURL := fmt.Sprintf("%s/invite/%s", s.platformURL, token)
		subject := fmt.Sprintf("You're invited to join %s on Observability Platform", companyName)
		body := s.emailSender.InviteTeammateHTML(inviterName, companyName, email, role, inviteURL)

		if err := s.emailSender.Send(email, subject, body); err != nil {
			log.Printf("[team] failed to send invite email to %s: %v", email, err)
		} else {
			log.Printf("[team] invite email sent to %s", email)
		}
	}

	return inv, nil
}

func (s *TeamService) AcceptInvitation(ctx context.Context, token string, fullName string, password string) error {
	inv, err := s.invitationRepo.FindByTokenCtx(ctx, token)
	if err != nil {
		return err
	}
	if inv == nil {
		return errors.New("invalid invitation token")
	}
	if inv.Status != "pending" {
		return errors.New("invitation is no longer pending")
	}
	if time.Now().After(inv.ExpiresAt) {
		return errors.New("invitation has expired")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		userRepo := s.userRepo.WithDB(tx)
		invRepo := s.invitationRepo.WithDB(tx)

		user := &domain.User{
			TenantID:     inv.TenantID,
			FullName:     fullName,
			Email:        inv.Email,
			PasswordHash: string(hash),
			Role:         inv.Role,
			Status:       string(domain.UserStatusActive),
		}
		if err := userRepo.Create(user); err != nil {
			return err
		}
		return invRepo.UpdateStatusCtx(ctx, inv.ID, "accepted")
	})
}

func (s *TeamService) ListTeam(ctx context.Context, tenantID uint) (*dto.TeamResponse, error) {
	users, err := s.userRepo.ListByTenantCtx(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	invites, err := s.invitationRepo.ListByTenantCtx(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	members := make([]dto.TeamMember, 0, len(users))
	for _, u := range users {
		status := u.Status
		if status == "" {
			status = string(domain.UserStatusActive)
		}
		members = append(members, dto.TeamMember{
			ID:        u.ID,
			FullName:  u.FullName,
			Email:     u.Email,
			Role:      u.Role,
			Status:    status,
			CreatedAt: u.CreatedAt.Format(time.RFC3339),
		})
	}

	pending := make([]dto.PendingInvite, 0, len(invites))
	for _, inv := range invites {
		inviteURL := fmt.Sprintf("%s/invite/%s", "", inv.Token)
		pending = append(pending, dto.PendingInvite{
			ID:        inv.ID,
			Email:     inv.Email,
			Role:      inv.Role,
			InviteURL: inviteURL,
			ExpiresAt: inv.ExpiresAt.Format(time.RFC3339),
			CreatedAt: inv.CreatedAt.Format(time.RFC3339),
		})
	}

	return &dto.TeamResponse{Members: members, Pending: pending}, nil
}

func (s *TeamService) RemoveMember(ctx context.Context, tenantID uint, targetUserID uint, callerUserID uint) error {
	if targetUserID == callerUserID {
		return errors.New("cannot remove yourself")
	}

	count, err := s.userRepo.CountByTenantAndRoleCtx(ctx, tenantID, "admin")
	if err != nil {
		return err
	}

	target, err := s.userRepo.FindByIDCtx(ctx, targetUserID)
	if err != nil {
		return err
	}
	if target == nil || target.TenantID != tenantID {
		return errors.New("user not found in this tenant")
	}

	if target.Role == "admin" && count <= 1 {
		return errors.New("cannot remove the last admin")
	}

	return s.userRepo.DeleteByIDCtx(ctx, targetUserID)
}

func (s *TeamService) ChangeRole(ctx context.Context, tenantID uint, targetUserID uint, newRole string) error {
	if newRole != "admin" && newRole != "member" {
		return apierror.ErrValidationFailed
	}

	target, err := s.userRepo.FindByIDCtx(ctx, targetUserID)
	if err != nil {
		return err
	}
	if target == nil || target.TenantID != tenantID {
		return errors.New("user not found in this tenant")
	}

	if target.Role == "admin" && newRole == "member" {
		count, err := s.userRepo.CountByTenantAndRoleCtx(ctx, tenantID, "admin")
		if err != nil {
			return err
		}
		if count <= 1 {
			return errors.New("cannot demote the last admin")
		}
	}

	return s.userRepo.UpdateRoleCtx(ctx, targetUserID, newRole)
}

// SetUserStatus activates or deactivates a team member account.
// Deactivated users are blocked at login/refresh and on every API call.
func (s *TeamService) SetUserStatus(ctx context.Context, tenantID uint, targetUserID uint, callerUserID uint, status string) error {
	if status != string(domain.UserStatusActive) && status != string(domain.UserStatusInactive) {
		return apierror.ErrValidationFailed
	}

	target, err := s.userRepo.FindByIDCtx(ctx, targetUserID)
	if err != nil {
		return err
	}
	if target == nil || target.TenantID != tenantID {
		return errors.New("user not found in this tenant")
	}

	if targetUserID == callerUserID {
		return errors.New("cannot change your own status")
	}

	// Prevent deactivating the last active admin — or no admin would be able
	// to manage the account anymore.
	if target.Role == "admin" && status == string(domain.UserStatusInactive) {
		count, err := s.userRepo.CountByTenantAndRoleAndStatusCtx(ctx, tenantID, "admin", string(domain.UserStatusActive))
		if err != nil {
			return err
		}
		if count <= 1 {
			return errors.New("cannot deactivate the last active admin")
		}
	}

	if target.Status == status {
		return nil
	}

	return s.userRepo.UpdateStatusCtx(ctx, targetUserID, status)
}

func (s *TeamService) ResendInvitation(ctx context.Context, invitationID uint, tenantID uint) (*domain.Invitation, error) {
	inv, err := s.invitationRepo.FindByIDAndTenantCtx(ctx, invitationID, tenantID)
	if err != nil {
		return nil, err
	}
	if inv == nil {
		return nil, errors.New("invitation not found")
	}
	if inv.Status != "pending" {
		return nil, errors.New("invitation is not pending")
	}

	token, err := generateInviteToken()
	if err != nil {
		return nil, err
	}

	inv.Token = token
	inv.ExpiresAt = time.Now().Add(72 * time.Hour)

	if err := s.db.WithContext(ctx).Model(&domain.Invitation{}).Where("id = ?", inv.ID).Updates(map[string]interface{}{
		"token":      inv.Token,
		"expires_at": inv.ExpiresAt,
	}).Error; err != nil {
		return nil, err
	}

	tenant, _ := s.tenantRepo.FindByIDCtx(ctx, tenantID)
	companyName := ""
	if tenant != nil {
		companyName = tenant.CompanyName
	}
	if companyName == "" {
		companyName = "the team"
	}
	inviteURL := fmt.Sprintf("%s/invite/%s", s.platformURL, token)
	subject := fmt.Sprintf("Reminder: You're invited to join %s on Observability Platform", companyName)
	body := s.emailSender.InviteTeammateHTML("A team admin", companyName, inv.Email, inv.Role, inviteURL)
	if err := s.emailSender.Send(inv.Email, subject, body); err != nil {
		log.Printf("[team] failed to resend invite email to %s: %v", inv.Email, err)
	} else {
		log.Printf("[team] invite email resent to %s", inv.Email)
	}

	return inv, nil
}

func (s *TeamService) ValidateInvitation(ctx context.Context, token string) (*domain.Invitation, error) {
	inv, err := s.invitationRepo.FindByTokenCtx(ctx, token)
	if err != nil {
		return nil, err
	}
	if inv == nil {
		return nil, errors.New("invalid invitation token")
	}
	if inv.Status != "pending" {
		return nil, errors.New("invitation is no longer pending")
	}
	if time.Now().After(inv.ExpiresAt) {
		return nil, errors.New("invitation has expired")
	}
	return inv, nil
}

func (s *TeamService) RevokeInvitation(ctx context.Context, invitationID uint, tenantID uint) error {
	inv, err := s.invitationRepo.FindByIDAndTenantCtx(ctx, invitationID, tenantID)
	if err != nil {
		return err
	}
	if inv == nil {
		return errors.New("invitation not found")
	}
	return s.invitationRepo.UpdateStatusCtx(ctx, inv.ID, "revoked")
}
