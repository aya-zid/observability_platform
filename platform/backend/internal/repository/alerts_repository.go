package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/dto"
)

type AlertsRepository struct {
	db *gorm.DB
}

func NewAlertsRepository(db *gorm.DB) *AlertsRepository {
	return &AlertsRepository{db: db}
}

type ScannableService struct {
	DiscoveredServiceID uint
	TenantID            uint
	MachineID           string
	ServiceName         string
	Version             string
}

func (r *AlertsRepository) ListVulnerabilityAlertsByTenantCtx(ctx context.Context, tenantID uint) ([]dto.VulnerabilityAlertListItem, error) {
	type row struct {
		ID             uint
		HostID         uint
		Hostname       string
		MachineID      string
		ServiceName    string
		ServiceVersion string
		CVEID          string
		CVSSScore      float32
		Severity       string
		Description    string
		FixedInVersion string
	}

	var rows []row
	err := r.db.WithContext(ctx).
		Table("service_vulnerabilities sv").
		Select("sv.id, sv.host_id, h.hostname, h.machine_id, sv.service_name, sv.service_version, sv.cve_id, sv.cvss_score, sv.severity, sv.description, sv.fixed_in_version").
		Joins("join hosts h on h.id = sv.host_id").
		Where("sv.tenant_id = ?", tenantID).
		Order("sv.cvss_score desc, h.hostname asc, sv.service_name asc, sv.cve_id asc").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	out := make([]dto.VulnerabilityAlertListItem, 0, len(rows))
	for _, row := range rows {
		alertLevel := "warning"
		if row.CVSSScore >= 7.0 || row.Severity == "HIGH" || row.Severity == "CRITICAL" {
			alertLevel = "alert"
		}
		out = append(out, dto.VulnerabilityAlertListItem{
			ID:             row.ID,
			HostID:         row.HostID,
			Hostname:       row.Hostname,
			MachineID:      row.MachineID,
			ServiceName:    row.ServiceName,
			ServiceVersion: row.ServiceVersion,
			CVEID:          row.CVEID,
			CVSSScore:      row.CVSSScore,
			Severity:       row.Severity,
			AlertLevel:     alertLevel,
			Description:    row.Description,
			FixedInVersion: row.FixedInVersion,
		})
	}
	return out, nil
}

func (r *AlertsRepository) ListServiceVulnerabilitiesCtx(ctx context.Context, tenantID uint, discoveredServiceID uint) ([]dto.ServiceVulnerabilityItem, error) {
	var rows []domain.ServiceVulnerability
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND discovered_service_id = ?", tenantID, discoveredServiceID).
		Order("cvss_score desc, cve_id asc").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	out := make([]dto.ServiceVulnerabilityItem, 0, len(rows))
	for _, row := range rows {
		alertLevel := "warning"
		if row.CVSSScore >= 7.0 || row.Severity == "HIGH" || row.Severity == "CRITICAL" {
			alertLevel = "alert"
		}
		out = append(out, dto.ServiceVulnerabilityItem{
			CVEID:          row.CVEID,
			CVSSScore:      row.CVSSScore,
			Severity:       row.Severity,
			AlertLevel:     alertLevel,
			Description:    row.Description,
			FixedInVersion: row.FixedInVersion,
		})
	}
	return out, nil
}

func (r *AlertsRepository) CountHighRiskVulnerableServicesCtx(ctx context.Context, tenantID uint) (int, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&domain.ServiceVulnerability{}).
		Where("tenant_id = ? AND (cvss_score >= ? OR upper(severity) IN ?)", tenantID, 7.0, []string{"HIGH", "CRITICAL"}).
		Distinct("discovered_service_id").
		Count(&count).Error
	return int(count), err
}

func (r *AlertsRepository) ListAllDiscoveredServicesCtx(ctx context.Context) ([]ScannableService, error) {
	var rows []ScannableService
	err := r.db.WithContext(ctx).
		Table("discovered_services").
		Select("id as discovered_service_id, tenant_id, machine_id, service_name, coalesce(version, '') as version").
		Order("tenant_id asc, machine_id asc, service_name asc").
		Scan(&rows).Error
	return rows, err
}

func (r *AlertsRepository) GetHostIDForServiceCtx(ctx context.Context, discoveredServiceID uint) (uint, error) {
	var ds domain.DiscoveredService
	if err := r.db.WithContext(ctx).
		Select("machine_id, tenant_id").
		First(&ds, discoveredServiceID).Error; err != nil {
		return 0, err
	}

	var row struct {
		ID uint
	}
	if err := r.db.WithContext(ctx).
		Table("hosts").
		Select("id").
		Where("tenant_id = ? AND machine_id = ?", ds.TenantID, ds.MachineID).
		Scan(&row).Error; err != nil {
		return 0, err
	}
	return row.ID, nil
}

func (r *AlertsRepository) SetScanStatusCtx(
	ctx context.Context,
	discoveredServiceID uint,
	scanSupported bool,
	status domain.VulnScanStatus,
	scanAt *time.Time,
) error {
	updates := map[string]interface{}{
		"vulnerability_scan_supported": scanSupported,
		"vulnerability_scan_status":    status,
		"updated_at":                   time.Now().UTC(),
	}
	if scanAt != nil {
		updates["last_vulnerability_scan_at"] = *scanAt
	}

	return r.db.WithContext(ctx).
		Model(&domain.DiscoveredService{}).
		Where("id = ?", discoveredServiceID).
		Updates(updates).Error
}

func (r *AlertsRepository) ReplaceServiceVulnerabilitiesCtx(
	ctx context.Context,
	tenantID uint,
	hostID uint,
	discoveredServiceID uint,
	serviceName string,
	serviceVersion string,
	items []domain.ServiceVulnerability,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.
			Where("tenant_id = ? AND host_id = ? AND discovered_service_id = ?", tenantID, hostID, discoveredServiceID).
			Delete(&domain.ServiceVulnerability{}).Error; err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}
		now := time.Now().UTC()
		for i := range items {
			items[i].TenantID = tenantID
			items[i].HostID = hostID
			items[i].DiscoveredServiceID = discoveredServiceID
			items[i].ServiceName = serviceName
			items[i].ServiceVersion = serviceVersion
			items[i].CheckedAt = now
			if err := tx.Create(&items[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
