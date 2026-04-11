package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
)

type HostRepository struct {
	db *gorm.DB
}

func NewHostRepository(db *gorm.DB) *HostRepository {
	return &HostRepository{db: db}
}

func (r *HostRepository) UpsertByTenantMachineCtx(
	ctx context.Context,
	tenantID uint,
	machineID, hostname, osName, osVersion, arch, agentVersion string,
) error {
	now := time.Now()
	host := domain.Host{
		TenantID:      tenantID,
		MachineID:     machineID,
		Hostname:      hostname,
		OSName:        osName,
		OSVersion:     osVersion,
		Arch:          arch,
		AgentVersion:  agentVersion,
		LastSeenAt:    now,
	}

	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "tenant_id"},
				{Name: "machine_id"},
			},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"hostname":      hostname,
				"os_name":       osName,
				"os_version":    osVersion,
				"arch":          arch,
				"agent_version": agentVersion,
				"last_seen_at":  now,
				"updated_at":    now,
			}),
		}).
		Create(&host).Error
}

func (r *HostRepository) TouchHeartbeatByTenantMachineCtx(
	ctx context.Context,
	tenantID uint,
	machineID, hostname, agentVersion string,
) error {
	now := time.Now()

	updates := map[string]interface{}{
		"last_seen_at": now,
		"updated_at":   now,
	}
	if hostname != "" {
		updates["hostname"] = hostname
	}
	if agentVersion != "" {
		updates["agent_version"] = agentVersion
	}

	tx := r.db.WithContext(ctx).
		Model(&domain.Host{}).
		Where("tenant_id = ? AND machine_id = ?", tenantID, machineID).
		Updates(updates)
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected > 0 {
		return nil
	}

	// If host was never registered (or cache/process race), create a minimal row.
	return r.UpsertByTenantMachineCtx(ctx, tenantID, machineID, hostname, "", "", "", agentVersion)
}

func (r *HostRepository) FindByTenantMachineCtx(ctx context.Context, tenantID uint, machineID string) (*domain.Host, error) {
	var host domain.Host
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND machine_id = ?", tenantID, machineID).
		First(&host).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &host, err
}

func (r *HostRepository) ListByTenantCtx(ctx context.Context, tenantID uint) ([]domain.Host, error) {
	var rows []domain.Host
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("hostname ASC").
		Find(&rows).Error
	return rows, err
}
