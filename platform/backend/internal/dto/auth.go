// internal/dto/auth.go
package dto

// SignupRequest - Input for POST /signup
type SignupRequest struct {
	CompanyName  string `json:"company_name" validate:"required,min=2,max=100"`
	ContactEmail string `json:"contact_email" validate:"required,email"`
	ContactPhone string `json:"contact_phone,omitempty"`
	FullName     string `json:"full_name" validate:"required,min=2,max=100"`
	Email        string `json:"email" validate:"required,email"`
	Password     string `json:"password" validate:"required,min=8"`
}

// LoginRequest - Input for POST /login
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// AuthResponse - Output for both signup and login
type AuthResponse struct {
	Token string `json:"token"`
}