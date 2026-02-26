package domain

import "time"

type User struct {
    ID           uint      `gorm:"primaryKey"`
    TenantID     uint      `gorm:"index"`               // ← foreign key here
    Tenant       Tenant    `gorm:"foreignKey:TenantID"` // ← belongs to (back reference)

    FullName     string    `gorm:"type:varchar(255)"`
    Email        string    `gorm:"type:varchar(255);unique;not null"`
    PasswordHash string    `gorm:"type:varchar(255);not null"`
    Role         string    `gorm:"type:varchar(50);default:'admin'"`

    CreatedAt    time.Time `gorm:"autoCreateTime"`
    UpdatedAt    time.Time `gorm:"autoUpdateTime"`
}