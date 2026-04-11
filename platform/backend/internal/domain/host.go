package domain

import "time"

type Host struct {
	ID uint `gorm:"primaryKey"`

	TenantID uint   `gorm:"not null;index;uniqueIndex:idx_hosts_tenant_machine"`
	Tenant   Tenant `gorm:"foreignKey:TenantID"`

	MachineID string `gorm:"type:varchar(255);not null;uniqueIndex:idx_hosts_tenant_machine"`
	Hostname  string `gorm:"type:varchar(255);not null"`

	OSName    string `gorm:"type:varchar(128)"`
	OSVersion string `gorm:"type:varchar(128)"`
	Arch      string `gorm:"type:varchar(64)"`

	AgentVersion string `gorm:"type:varchar(64)"`
	LastSeenAt   time.Time `gorm:"not null;index"`

	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

