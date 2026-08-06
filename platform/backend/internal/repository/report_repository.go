package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
)

type ReportRepository struct {
	db *gorm.DB
}

func NewReportRepository(db *gorm.DB) *ReportRepository {
	return &ReportRepository{db: db}
}

func (r *ReportRepository) Create(ctx context.Context, report *domain.Report) error {
	return r.db.WithContext(ctx).Create(report).Error
}

// FindReadyByKey returns an existing ready report for the same dedup key
// (tenant, type, date_a, date_b), or nil if none exists.
func (r *ReportRepository) FindReadyByKey(ctx context.Context, tenantID uint, typ, dateA, dateB string) (*domain.Report, error) {
	var report domain.Report
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND type = ? AND date_a = ? AND date_b = ? AND status = 'ready'", tenantID, typ, dateA, dateB).
		First(&report).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &report, nil
}

func (r *ReportRepository) GetByIDTenant(ctx context.Context, id uint, tenantID uint) (*domain.Report, error) {
	var report domain.Report
	err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		First(&report).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &report, nil
}

func (r *ReportRepository) ListByTenant(ctx context.Context, tenantID uint, typ string, limit, offset int) ([]domain.Report, error) {
	if limit <= 0 {
		limit = 50
	}
	q := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID)
	if typ != "" {
		q = q.Where("type = ?", typ)
	}
	var rows []domain.Report
	err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&rows).Error
	return rows, err
}

func (r *ReportRepository) MarkReady(ctx context.Context, id uint, numbers, aiReport json.RawMessage) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&domain.Report{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":      "ready",
			"numbers":     numbers,
			"ai_report":   aiReport,
			"error":       "",
			"finished_at": &now,
		}).Error
}

// MarkGenerating resets a report back to the generating state and clears any
// previous error. Used by Regenerate.
func (r *ReportRepository) MarkGenerating(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).
		Model(&domain.Report{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":      "generating",
			"error":       "",
			"finished_at": nil,
		}).Error
}

func (r *ReportRepository) MarkError(ctx context.Context, id uint, errMsg string) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&domain.Report{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":      "error",
			"error":       errMsg,
			"finished_at": &now,
		}).Error
}

func (r *ReportRepository) Delete(ctx context.Context, id uint, tenantID uint) error {
	return r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&domain.Report{}).Error
}
