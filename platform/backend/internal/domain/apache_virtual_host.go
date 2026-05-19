// internal/domain/apache_virtual_host.go
package domain

import "time"

// ApacheVirtualHost maps one log file to one Apache virtual host domain.
// One row per (discovered_service_id, domain, log_kind, log_path).
type ApacheVirtualHost struct {
	ID uint `gorm:"primaryKey"`

	TenantID uint `gorm:"not null;index;uniqueIndex:idx_apache_vhost_service_domain_kind_path"`

	DiscoveredServiceID uint              `gorm:"not null;index;uniqueIndex:idx_apache_vhost_service_domain_kind_path"`
	DiscoveredService   DiscoveredService `gorm:"foreignKey:DiscoveredServiceID;constraint:OnDelete:CASCADE"`

	Domain  string `gorm:"type:varchar(255);not null;uniqueIndex:idx_apache_vhost_service_domain_kind_path"`
	LogKind string `gorm:"type:varchar(20);not null;uniqueIndex:idx_apache_vhost_service_domain_kind_path"`
	LogPath string `gorm:"type:text;not null;uniqueIndex:idx_apache_vhost_service_domain_kind_path"`

	Source      string `gorm:"type:varchar(20);not null;default:'detected'"`
	Enabled     bool   `gorm:"type:boolean;not null;default:true"`
	IsSharedLog bool   `gorm:"type:boolean;not null;default:false"`
	IsDefault   bool   `gorm:"type:boolean;not null;default:false"`

	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

func (ApacheVirtualHost) TableName() string {
	return "apache_virtual_hosts"
}
