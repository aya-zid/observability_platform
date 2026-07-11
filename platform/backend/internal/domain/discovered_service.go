package domain

import "time"

type VulnScanStatus string

const (
	VulnScanStatusNotScanned  VulnScanStatus = "not_scanned"
	VulnScanStatusScanned     VulnScanStatus = "scanned"
	VulnScanStatusUnsupported VulnScanStatus = "unsupported"
	VulnScanStatusFailed      VulnScanStatus = "failed"
	VulnScanStatusNoVersion   VulnScanStatus = "no_version"
)

type DiscoveredService struct {
	ID uint `gorm:"primaryKey"`

	TenantID  uint   `gorm:"not null;index;uniqueIndex:idx_disc_service_tenant_machine_name"`
	MachineID string `gorm:"type:varchar(255);not null;uniqueIndex:idx_disc_service_tenant_machine_name"`

	ServiceName string `gorm:"type:varchar(120);not null;uniqueIndex:idx_disc_service_tenant_machine_name"`
	Port        *int   `gorm:"index"`
	Version     string `gorm:"type:varchar(255)"`

	AccessLogPath     string `gorm:"type:text"`
	ErrorLogPath      string `gorm:"type:text"`
	ConfirmedLogPath  string `gorm:"type:text"`
	LogPathSource     string `gorm:"type:varchar(20);not null;default:'detected'"`
	UserMetricsEndpoint string `gorm:"type:text"`

	LoggingMode  string `gorm:"type:varchar(20);default:'unknown'"` // file|syslog|journald|mixed|unknown
	UsesJournald bool   `gorm:"type:boolean;not null;default:false"`
	UsesSyslog   bool   `gorm:"type:boolean;not null;default:false"`

	LogCollectionOn    bool   `gorm:"type:boolean;not null;default:false"`
	MetricCollectionOn bool   `gorm:"type:boolean;not null;default:false"`

	VulnerabilityScanSupported bool           `gorm:"type:boolean;not null;default:false"`
	VulnerabilityScanStatus    VulnScanStatus `gorm:"type:varchar(20);not null;default:'not_scanned'"`
	LastVulnerabilityScanAt    *time.Time     `gorm:"index"`

	LastSeenAt time.Time `gorm:"not null;index"`
	CreatedAt  time.Time `gorm:"autoCreateTime"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime"`
}

type ServiceVulnerability struct {
	ID uint `gorm:"primaryKey"`

	TenantID uint `gorm:"not null;index"`
	HostID   uint `gorm:"not null;index"`

	DiscoveredServiceID uint              `gorm:"not null;index"`
	DiscoveredService   DiscoveredService `gorm:"foreignKey:DiscoveredServiceID;constraint:OnDelete:CASCADE"`

	ServiceName    string    `gorm:"type:varchar(120);not null;index"`
	ServiceVersion string    `gorm:"type:varchar(255)"`
	CVEID          string    `gorm:"type:varchar(40);not null;index"`
	CVSSScore      float32
	Severity       string    `gorm:"type:varchar(20)"`
	Description    string    `gorm:"type:text"`
	FixedInVersion string    `gorm:"type:varchar(255)"`
	PublishedAt    time.Time
	CheckedAt      time.Time `gorm:"not null;index"`

	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

type DiscoveredServiceLogPath struct {
	ID uint `gorm:"primaryKey"`

	TenantID uint `gorm:"not null;index"`

	DiscoveredServiceID uint              `gorm:"not null;index;uniqueIndex:idx_disc_service_path"`
	DiscoveredService   DiscoveredService `gorm:"foreignKey:DiscoveredServiceID;constraint:OnDelete:CASCADE"`

	Path string `gorm:"type:text;not null;uniqueIndex:idx_disc_service_path"`

	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}
