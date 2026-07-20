package domain

type TenantUsageDaily struct {
	TenantID              uint   `gorm:"primaryKey;autoIncrement:false"`
	Date                  string `gorm:"primaryKey;type:date"`
	LogBytesIngested      int64  `gorm:"default:0"`
	MetricSamplesIngested int64  `gorm:"default:0"`
}
