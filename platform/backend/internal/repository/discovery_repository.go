// internal/repository/discovery_repository.go
package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
)

type DiscoveryRepository struct {
	db *gorm.DB
}

func NewDiscoveryRepository(db *gorm.DB) *DiscoveryRepository {
	return &DiscoveryRepository{db: db}
}

func (r *DiscoveryRepository) WithDB(db *gorm.DB) *DiscoveryRepository {
	return &DiscoveryRepository{db: db}
}

func (r *DiscoveryRepository) UpsertServiceCtx(ctx context.Context, rec domain.DiscoveredService) (domain.DiscoveredService, error) {
	now := time.Now()
	rec.LastSeenAt = now

	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "tenant_id"},
				{Name: "machine_id"},
				{Name: "service_name"},
			},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"port":            rec.Port,
				"version":         rec.Version,
				"access_log_path": rec.AccessLogPath,
				"error_log_path":  rec.ErrorLogPath,
				"logging_mode":    rec.LoggingMode,
				"uses_journald":   rec.UsesJournald,
				"uses_syslog":     rec.UsesSyslog,
				"last_seen_at":    now,
				"updated_at":      now,
			}),
		}).
		Create(&rec).Error
	if err != nil {
		return domain.DiscoveredService{}, err
	}

	var out domain.DiscoveredService
	err = r.db.WithContext(ctx).
		Where("tenant_id = ? AND machine_id = ? AND service_name = ?", rec.TenantID, rec.MachineID, rec.ServiceName).
		First(&out).Error
	return out, err
}

func (r *DiscoveryRepository) UpdateServiceConfigCtx(
	ctx context.Context,
	tenantID uint,
	serviceID uint,
	confirmedLogPath string,
	logPathSource string,
	userMetricsEndpoint string,
) error {
	tx := r.db.WithContext(ctx).
		Model(&domain.DiscoveredService{}).
		Where("id = ? AND tenant_id = ?", serviceID, tenantID).
		Updates(map[string]interface{}{
			"confirmed_log_path":    confirmedLogPath,
			"log_path_source":       logPathSource,
			"user_metrics_endpoint": userMetricsEndpoint,
			"updated_at":            time.Now(),
		})
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *DiscoveryRepository) ReplaceServiceLogPathsCtx(ctx context.Context, tenantID uint, serviceID uint, paths []string) error {
	if err := r.db.WithContext(ctx).
		Where("discovered_service_id = ?", serviceID).
		Delete(&domain.DiscoveredServiceLogPath{}).Error; err != nil {
		return err
	}
	if len(paths) == 0 {
		return nil
	}
	for _, p := range paths {
		if p == "" {
			continue
		}
		row := domain.DiscoveredServiceLogPath{
			TenantID:            tenantID,
			DiscoveredServiceID: serviceID,
			Path:                p,
		}
		if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *DiscoveryRepository) TouchServiceHeartbeatCtx(ctx context.Context, tenantID uint, machineID string, serviceNames []string) error {
	if len(serviceNames) == 0 {
		return nil
	}
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&domain.DiscoveredService{}).
		Where("tenant_id = ? AND machine_id = ? AND service_name IN ?", tenantID, machineID, serviceNames).
		Updates(map[string]interface{}{
			"last_seen_at": now,
			"updated_at":   now,
		}).Error
}

func (r *DiscoveryRepository) TouchContainerHeartbeatCtx(ctx context.Context, tenantID uint, machineID string, containerNames []string) error {
	if len(containerNames) == 0 {
		return nil
	}
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&domain.DiscoveredContainerService{}).
		Where("tenant_id = ? AND machine_id = ? AND container_name IN ?", tenantID, machineID, containerNames).
		Updates(map[string]interface{}{
			"last_seen_at": now,
			"updated_at":   now,
		}).Error
}

func (r *DiscoveryRepository) UpsertContainerServiceCtx(ctx context.Context, rec domain.DiscoveredContainerService) error {
	now := time.Now()
	rec.LastSeenAt = now

	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "tenant_id"},
				{Name: "machine_id"},
				{Name: "container_name"},
			},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"service_name":    rec.ServiceName,
				"container_image": rec.ContainerImage,
				"last_seen_at":    now,
				"updated_at":      now,
			}),
		}).
		Create(&rec).Error
}

func (r *DiscoveryRepository) ListContainerServicesByTenantMachineCtx(ctx context.Context, tenantID uint, machineID string) ([]domain.DiscoveredContainerService, error) {
	var rows []domain.DiscoveredContainerService
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND machine_id = ?", tenantID, machineID).
		Order("container_name ASC").
		Find(&rows).Error
	return rows, err
}

func (r *DiscoveryRepository) ListContainerServicesByTenantCtx(ctx context.Context, tenantID uint) ([]domain.DiscoveredContainerService, error) {
	var rows []domain.DiscoveredContainerService
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("machine_id ASC, container_name ASC").
		Find(&rows).Error
	return rows, err
}

func (r *DiscoveryRepository) FindContainerByNameMachineCtx(ctx context.Context, tenantID uint, machineID string, containerName string) (*domain.DiscoveredContainerService, error) {
	var row domain.DiscoveredContainerService
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND machine_id = ? AND container_name = ?", tenantID, machineID, containerName).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *DiscoveryRepository) FindContainerServiceByIDTenantCtx(ctx context.Context, tenantID uint, serviceID uint) (*domain.DiscoveredContainerService, error) {
	var row domain.DiscoveredContainerService
	err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", serviceID, tenantID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *DiscoveryRepository) ListServicesByTenantMachineCtx(ctx context.Context, tenantID uint, machineID string) ([]domain.DiscoveredService, error) {
	var rows []domain.DiscoveredService
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND machine_id = ?", tenantID, machineID).
		Order("service_name ASC").
		Find(&rows).Error
	return rows, err
}

func (r *DiscoveryRepository) ListServicesByTenantCtx(ctx context.Context, tenantID uint) ([]domain.DiscoveredService, error) {
	var rows []domain.DiscoveredService
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("machine_id ASC, service_name ASC").
		Find(&rows).Error
	return rows, err
}

func (r *DiscoveryRepository) FindServiceByIDTenantCtx(ctx context.Context, tenantID uint, serviceID uint) (*domain.DiscoveredService, error) {
	var row domain.DiscoveredService
	err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", serviceID, tenantID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *DiscoveryRepository) ListServiceLogPathsCtx(ctx context.Context, tenantID uint, serviceID uint) ([]string, error) {
	var rows []domain.DiscoveredServiceLogPath
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND discovered_service_id = ?", tenantID, serviceID).
		Order("id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.Path)
	}
	return out, nil
}

func (r *DiscoveryRepository) UpdateServiceCollectionCtx(
	ctx context.Context,
	tenantID uint,
	serviceID uint,
	logCollectionOn bool,
	metricCollectionOn bool,
) error {
	tx := r.db.WithContext(ctx).
		Model(&domain.DiscoveredService{}).
		Where("id = ? AND tenant_id = ?", serviceID, tenantID).
		Updates(map[string]interface{}{
			"log_collection_on":    logCollectionOn,
			"metric_collection_on": metricCollectionOn,
			"updated_at":           time.Now(),
		})
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *DiscoveryRepository) UpdateServiceCollectionByMachineNameCtx(
	ctx context.Context,
	tenantID uint,
	machineID string,
	serviceName string,
	logCollectionOn bool,
	metricCollectionOn bool,
) error {
	tx := r.db.WithContext(ctx).
		Model(&domain.DiscoveredService{}).
		Where("tenant_id = ? AND machine_id = ? AND service_name = ?", tenantID, machineID, serviceName).
		Updates(map[string]interface{}{
			"log_collection_on":    logCollectionOn,
			"metric_collection_on": metricCollectionOn,
			"updated_at":           time.Now(),
		})
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ── Nginx Virtual Host methods ────────────────────────────────────────────────

// ListVirtualHostsByServiceCtx returns all nginx virtual host rows for a service,
// ordered by domain then log_kind. Callers group by domain before returning to the UI.
func (r *DiscoveryRepository) ListVirtualHostsByServiceCtx(
	ctx context.Context,
	tenantID uint,
	serviceID uint,
) ([]domain.NginxVirtualHost, error) {
	var rows []domain.NginxVirtualHost
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND discovered_service_id = ?", tenantID, serviceID).
		Order("domain ASC, log_kind ASC").
		Find(&rows).Error
	return rows, err
}

// ReplaceVirtualHostsCtx deletes all existing virtual hosts for a service and
// inserts the provided list. Used when the user saves the virtual hosts table in the UI.
func (r *DiscoveryRepository) ReplaceVirtualHostsCtx(
	ctx context.Context,
	tenantID uint,
	serviceID uint,
	vhosts []domain.NginxVirtualHost,
) error {
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND discovered_service_id = ?", tenantID, serviceID).
		Delete(&domain.NginxVirtualHost{}).Error; err != nil {
		return err
	}
	if len(vhosts) == 0 {
		return nil
	}
	for i := range vhosts {
		vhosts[i].TenantID = tenantID
		vhosts[i].DiscoveredServiceID = serviceID
		if err := r.db.WithContext(ctx).Create(&vhosts[i]).Error; err != nil {
			return err
		}
	}
	return nil
}

// UpsertDetectedVirtualHostsCtx inserts agent-detected virtual hosts.
// On conflict (same tenant + service + domain + log_kind + log_path) it only
// bumps updated_at, preserving any user edits (source, enabled, is_shared_log).
//
// The OnConflict columns MUST exactly match the unique index
// idx_nginx_vhost_service_domain_kind_path defined in NginxVirtualHost:
//
//	tenant_id, discovered_service_id, domain, log_kind, log_path
func (r *DiscoveryRepository) UpsertDetectedVirtualHostsCtx(
	ctx context.Context,
	tenantID uint,
	serviceID uint,
	detectedVhosts []domain.NginxVirtualHost,
) error {
	now := time.Now()
	for i := range detectedVhosts {
		detectedVhosts[i].TenantID = tenantID
		detectedVhosts[i].DiscoveredServiceID = serviceID
		detectedVhosts[i].Source = "detected"

		err := r.db.WithContext(ctx).
			Clauses(clause.OnConflict{
				// These five columns match the unique index exactly.
				Columns: []clause.Column{
					{Name: "tenant_id"},
					{Name: "discovered_service_id"},
					{Name: "domain"},
					{Name: "log_kind"},
					{Name: "log_path"},
				},
				// On conflict: only bump updated_at.
				// Do NOT overwrite enabled/is_shared_log/source — the user may have
				// changed those in the UI.
				DoUpdates: clause.Assignments(map[string]interface{}{
					"updated_at": now,
				}),
			}).
			Create(&detectedVhosts[i]).Error
		if err != nil {
			return err
		}
	}
	return nil
}

// ── Apache Virtual Host methods ───────────────────────────────────────────────

func (r *DiscoveryRepository) ListApacheVirtualHostsByServiceCtx(
	ctx context.Context,
	tenantID uint,
	serviceID uint,
) ([]domain.ApacheVirtualHost, error) {
	var rows []domain.ApacheVirtualHost
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND discovered_service_id = ?", tenantID, serviceID).
		Order("domain ASC, log_kind ASC").
		Find(&rows).Error
	return rows, err
}

func (r *DiscoveryRepository) ReplaceApacheVirtualHostsCtx(
	ctx context.Context,
	tenantID uint,
	serviceID uint,
	vhosts []domain.ApacheVirtualHost,
) error {
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND discovered_service_id = ?", tenantID, serviceID).
		Delete(&domain.ApacheVirtualHost{}).Error; err != nil {
		return err
	}
	if len(vhosts) == 0 {
		return nil
	}
	for i := range vhosts {
		vhosts[i].TenantID = tenantID
		vhosts[i].DiscoveredServiceID = serviceID
		if err := r.db.WithContext(ctx).Create(&vhosts[i]).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *DiscoveryRepository) UpsertDetectedApacheVirtualHostsCtx(
	ctx context.Context,
	tenantID uint,
	serviceID uint,
	detectedVhosts []domain.ApacheVirtualHost,
) error {
	now := time.Now()
	for i := range detectedVhosts {
		detectedVhosts[i].TenantID = tenantID
		detectedVhosts[i].DiscoveredServiceID = serviceID
		detectedVhosts[i].Source = "detected"

		err := r.db.WithContext(ctx).
			Clauses(clause.OnConflict{
				Columns: []clause.Column{
					{Name: "tenant_id"},
					{Name: "discovered_service_id"},
					{Name: "domain"},
					{Name: "log_kind"},
					{Name: "log_path"},
				},
				DoUpdates: clause.Assignments(map[string]interface{}{
					"updated_at": now,
				}),
			}).
			Create(&detectedVhosts[i]).Error
		if err != nil {
			return err
		}
	}
	return nil
}
