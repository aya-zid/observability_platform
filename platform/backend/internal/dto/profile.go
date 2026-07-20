package dto

type UpdateProfileRequest struct {
	FullName string `json:"full_name" validate:"required,min=2,max=100"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required,min=8"`
}
