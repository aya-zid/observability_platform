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
				"port":                 rec.Port,
				"version":              rec.Version,
				"access_log_path":      rec.AccessLogPath,
				"error_log_path":       rec.ErrorLogPath,
				"logging_mode":         rec.LoggingMode,
				"uses_journald":        rec.UsesJournald,
				"uses_syslog":          rec.UsesSyslog,
				"last_seen_at":         now,
				"updated_at":           now,
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
	metricsMode string,
	isJavaApp bool,
	jmxExporterPort *int,
) error {
	tx := r.db.WithContext(ctx).
		Model(&domain.DiscoveredService{}).
		Where("id = ? AND tenant_id = ?", serviceID, tenantID).
		Updates(map[string]interface{}{
			"confirmed_log_path":   confirmedLogPath,
			"log_path_source":      logPathSource,
			"user_metrics_endpoint": userMetricsEndpoint,
			"metrics_mode":          metricsMode,
			"is_java_app":           isJavaApp,
			"jmx_exporter_port":     jmxExporterPort,
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
	status domain.IntegrationStatus,
) error {
	tx := r.db.WithContext(ctx).
		Model(&domain.DiscoveredService{}).
		Where("id = ? AND tenant_id = ?", serviceID, tenantID).
		Updates(map[string]interface{}{
			"log_collection_on":    logCollectionOn,
			"metric_collection_on": metricCollectionOn,
			"integration_status":   status,
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
	status domain.IntegrationStatus,
) error {
	tx := r.db.WithContext(ctx).
		Model(&domain.DiscoveredService{}).
		Where("tenant_id = ? AND machine_id = ? AND service_name = ?", tenantID, machineID, serviceName).
		Updates(map[string]interface{}{
			"log_collection_on":    logCollectionOn,
			"metric_collection_on": metricCollectionOn,
			"integration_status":   status,
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
