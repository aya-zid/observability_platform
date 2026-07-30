package repository

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
)

type IncidentRepository struct {
	db *gorm.DB
}

func NewIncidentRepository(db *gorm.DB) *IncidentRepository {
	return &IncidentRepository{db: db}
}

func (r *IncidentRepository) Create(ctx context.Context, inc *domain.Incident) error {
	return r.db.WithContext(ctx).Create(inc).Error
}

func (r *IncidentRepository) Update(ctx context.Context, inc *domain.Incident) error {
	return r.db.WithContext(ctx).Save(inc).Error
}

func (r *IncidentRepository) FindByID(ctx context.Context, id uint) (*domain.Incident, error) {
	var inc domain.Incident
	err := r.db.WithContext(ctx).Preload("Signals").First(&inc, id).Error
	if err != nil {
		return nil, err
	}
	return &inc, nil
}

func (r *IncidentRepository) ListActiveByTenant(ctx context.Context, tenantID uint) ([]domain.Incident, error) {
	var incidents []domain.Incident
	err := r.db.WithContext(ctx).
		Preload("Signals").
		Where("tenant_id = ? AND status = 'firing'", tenantID).
		Order("started_at DESC").
		Find(&incidents).Error
	return incidents, err
}

func (r *IncidentRepository) ListByTenant(ctx context.Context, tenantID uint, limit, offset int) ([]domain.Incident, error) {
	var incidents []domain.Incident
	err := r.db.WithContext(ctx).
		Preload("Signals").
		Where("tenant_id = ?", tenantID).
		Order("started_at DESC").
		Limit(limit).Offset(offset).
		Find(&incidents).Error
	return incidents, err
}

func (r *IncidentRepository) ResolveIncident(ctx context.Context, id uint) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&domain.Incident{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":      "resolved",
			"resolved_at": &now,
		}).Error
}

func (r *IncidentRepository) ResolveStaleByMachine(ctx context.Context, tenantID uint, machineID string, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	res := r.db.WithContext(ctx).
		Model(&domain.Incident{}).
		Where("tenant_id = ? AND machine_id = ? AND status = 'firing' AND last_seen_at < ?", tenantID, machineID, cutoff).
		Update("status", "resolved")
	return res.RowsAffected, res.Error
}

func (r *IncidentRepository) UpsertSignal(ctx context.Context, signal *domain.IncidentSignal) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "incident_id"}, {Name: "role"}, {Name: "source"}, {Name: "service_name"}, {Name: "metric_name"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "baseline_value", "log_evidence", "updated_at"}),
	}).Create(signal).Error
}

// ── DismissedIncident ──────────────────────────────────────────────────────────

func (r *IncidentRepository) DismissIncident(ctx context.Context, tenantID uint, incidentID string) error {
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND incident_id = ?", tenantID, incidentID).
		FirstOrCreate(&domain.DismissedIncident{
			TenantID:   tenantID,
			IncidentID: incidentID,
		}).Error
}

func (r *IncidentRepository) ListDismissedIncidents(ctx context.Context, tenantID uint) ([]string, error) {
	var rows []domain.DismissedIncident
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.IncidentID
	}
	return ids, nil
}