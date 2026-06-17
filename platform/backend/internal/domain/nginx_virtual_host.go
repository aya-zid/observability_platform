// internal/domain/nginx_virtual_host.go
package domain

import "time"

// NginxVirtualHost maps one log file to one virtual host domain.
// One row per (discovered_service_id, domain, log_kind, log_path).
//
// Unique index includes tenant_id so the OnConflict clause in
// UpsertDetectedVirtualHostsCtx can match it exactly.
type NginxVirtualHost struct {
	ID uint `gorm:"primaryKey"`

	TenantID uint `gorm:"not null;index;uniqueIndex:idx_nginx_vhost_service_domain_kind_path"`

	DiscoveredServiceID uint              `gorm:"not null;index;uniqueIndex:idx_nginx_vhost_service_domain_kind_path"`
	DiscoveredService   DiscoveredService `gorm:"foreignKey:DiscoveredServiceID;constraint:OnDelete:CASCADE"`

	// Domain identifies the virtual host (e.g., "shop.example.com").
	// For shared logs or catch-all servers, use "(default)".
	Domain string `gorm:"type:varchar(255);not null;uniqueIndex:idx_nginx_vhost_service_domain_kind_path"`

	// LogKind is "access" or "error".
	LogKind string `gorm:"type:varchar(20);not null;uniqueIndex:idx_nginx_vhost_service_domain_kind_path"`

	// LogPath is the absolute file path for this log.
	LogPath string `gorm:"type:text;not null;uniqueIndex:idx_nginx_vhost_service_domain_kind_path"`

	// Source indicates origin: "detected" (agent discovery) or "user" (manually edited in UI).
	Source string `gorm:"type:varchar(20);not null;default:'detected'"`

	// Enabled controls whether this row is included in the OTel collector config.
	Enabled bool `gorm:"type:boolean;not null"`

	// IsSharedLog is true when multiple domains write to the same log file.
	// The platform will parse the host= field to split traffic by domain.
	IsSharedLog bool `gorm:"type:boolean;not null;default:false"`

	// IsDefault is true for the catch-all / default_server block.
	IsDefault bool `gorm:"type:boolean;not null;default:false"`

	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

func (NginxVirtualHost) TableName() string {
	return "nginx_virtual_hosts"
}