package dto

type InviteRequest struct {
	Email string `json:"email" validate:"required,email"`
	Role  string `json:"role" validate:"required,oneof=admin member"`
}

type AcceptInviteRequest struct {
	FullName string `json:"full_name" validate:"required,min=2,max=100"`
	Password string `json:"password" validate:"required,min=8"`
}

type ChangeRoleRequest struct {
	Role string `json:"role" validate:"required,oneof=admin member"`
}

type UpdateUserStatusRequest struct {
	Status string `json:"status" validate:"required,oneof=active inactive"`
}

type TeamMember struct {
	ID        uint   `json:"id"`
	FullName  string `json:"full_name"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

type PendingInvite struct {
	ID        uint   `json:"id"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	InviteURL string `json:"invite_url"`
	ExpiresAt string `json:"expires_at"`
	CreatedAt string `json:"created_at"`
}

type TeamResponse struct {
	Members []TeamMember    `json:"members"`
	Pending []PendingInvite `json:"pending"`
}
