package dto

type UpdateOrgRequest struct {
	CompanyName string `json:"company_name" validate:"omitempty,min=2,max=100"`
	Timezone    string `json:"timezone" validate:"omitempty,max=50"`
}

type OrgSettingsResponse struct {
	CompanyName  string `json:"company_name"`
	ContactEmail string `json:"contact_email"`
	Timezone     string `json:"timezone"`
	CreatedAt    string `json:"created_at"`
}
