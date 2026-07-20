// internal/dto/auth.go
package dto

// SignupRequest - Input for POST /signup
type SignupRequest struct {
	CompanyName   string `json:"company_name" validate:"required,min=2,max=100"`
	BusinessEmail string `json:"business_email" validate:"required,email"`
	ContactPhone  string `json:"contact_phone,omitempty"`
	FullName      string `json:"full_name" validate:"required,min=2,max=100"`
	Password      string `json:"password" validate:"required,min=8,containsany=ABCDEFGHIJKLMNOPQRSTUVWXYZ,containsany=abcdefghijklmnopqrstuvwxyz,containsany=0123456789"`
}

// LoginRequest - Input for POST /login
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type APIKeyResponse struct {
	APIKey string `json:"api_key"`
}

type MeResponse struct {
	UserID      uint   `json:"user_id"`
	TenantID    uint   `json:"tenant_id"`
	FullName    string `json:"full_name"`
	Email       string `json:"email"`
	Role        string `json:"role"`
	CompanyName string `json:"company_name"`
}

type AuthResponse struct {
	OK   bool   `json:"ok"`
	Role string `json:"role"`
}

type OkResponse struct {
	OK bool `json:"ok"`
}
