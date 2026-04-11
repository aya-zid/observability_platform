package domain

import "time"

type DiscoveredContainerService struct {
	ID uint `gorm:"primaryKey"`

	TenantID  uint   `gorm:"not null;index;uniqueIndex:idx_disc_container_tenant_machine_container"`
	MachineID string `gorm:"type:varchar(255);not null;uniqueIndex:idx_disc_container_tenant_machine_container"`

	ContainerName string `gorm:"type:varchar(160);not null;uniqueIndex:idx_disc_container_tenant_machine_container"`
	ServiceName   string `gorm:"type:varchar(120);not null"` // detected service kind (nginx/mysql/unknown...)
	ContainerImage string `gorm:"type:varchar(255)"`

	LogCollectionOn    bool              `gorm:"type:boolean;not null;default:false"`
	MetricCollectionOn bool              `gorm:"type:boolean;not null;default:false"`
	IntegrationStatus  IntegrationStatus `gorm:"type:varchar(30);not null;default:'not_installed'"`

	LastSeenAt time.Time `gorm:"not null;index"`
	CreatedAt  time.Time `gorm:"autoCreateTime"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime"`
}

