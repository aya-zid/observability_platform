package domain

import "time"

type Tenant struct {
    ID           uint      `gorm:"primaryKey"`
    UUID         string    `gorm:"type:uuid;uniqueIndex"`
    CompanyName  string    `gorm:"type:varchar(255);not null"`
    ContactEmail string    `gorm:"type:varchar(255);unique;not null"`
    ContactPhone string    `gorm:"type:varchar(50)"`
    APIKey       string    `gorm:"type:varchar(100);unique;not null"` // later generated

    // Has many Users (bidirectional)
    Users        []User    `gorm:"foreignKey:TenantID"` 

    CreatedAt    time.Time `gorm:"autoCreateTime"`
    UpdatedAt    time.Time `gorm:"autoUpdateTime"`
}