package domain

type AlertTemplate struct {
	ID                 string
	ServiceType        string
	Name               string
	Description        string
	Unit               string
	DefaultThreshold   float64
	ComparisonOp       string
	DefaultSeverity    string
	DefaultForDuration string
}

var AlertTemplates = []AlertTemplate{
	{ID: "host_cpu", ServiceType: "host", Name: "High CPU usage", DefaultThreshold: 90, Unit: "%", Description: "Fires when average CPU usage exceeds threshold", DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m"},
	{ID: "host_memory", ServiceType: "host", Name: "High memory usage", DefaultThreshold: 85, Unit: "%", Description: "Fires when memory usage exceeds threshold", DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m"},
	{ID: "host_disk", ServiceType: "host", Name: "Disk space critical", DefaultThreshold: 90, Unit: "%", Description: "Fires when any disk partition usage exceeds threshold", DefaultSeverity: "critical", ComparisonOp: ">", DefaultForDuration: "5m"},
	{ID: "nginx_5xx", ServiceType: "nginx", Name: "High 5xx rate", DefaultThreshold: 5, Unit: "%", Description: "Fires when 5xx error rate exceeds threshold", DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m"},
	{ID: "nginx_p99", ServiceType: "nginx", Name: "High P99 latency", DefaultThreshold: 5, Unit: "seconds", Description: "Fires when 99th percentile response time exceeds threshold", DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m"},
	{ID: "apache_5xx", ServiceType: "apache", Name: "High 5xx rate", DefaultThreshold: 5, Unit: "%", Description: "Fires when 5xx error rate exceeds threshold", DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m"},
	{ID: "apache_p99", ServiceType: "apache", Name: "High P99 latency", DefaultThreshold: 5, Unit: "seconds", Description: "Fires when 99th percentile response time exceeds threshold", DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m"},
	{ID: "postgres_connections", ServiceType: "postgres", Name: "Connection saturation", DefaultThreshold: 80, Unit: "%", Description: "Fires when active connections exceed threshold percentage of max", DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m"},
	{ID: "postgres_deadlocks", ServiceType: "postgres", Name: "High deadlock rate", DefaultThreshold: 1, Unit: "count/min", Description: "Fires when deadlock rate exceeds threshold", DefaultSeverity: "critical", ComparisonOp: ">", DefaultForDuration: "5m"},
	{ID: "mariadb_slow_queries", ServiceType: "mariadb", Name: "Slow query spike", DefaultThreshold: 10, Unit: "count/min", Description: "Fires when slow query rate exceeds threshold", DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m"},
	{ID: "mariadb_connections", ServiceType: "mariadb", Name: "Connection saturation", DefaultThreshold: 80, Unit: "%", Description: "Fires when active connections exceed threshold percentage of max", DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m"},
	{ID: "redis_memory", ServiceType: "redis", Name: "High memory usage", DefaultThreshold: 80, Unit: "%", Description: "Fires when Redis memory usage exceeds threshold", DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m"},
	{ID: "redis_evictions", ServiceType: "redis", Name: "High eviction rate", DefaultThreshold: 100, Unit: "count/min", Description: "Fires when key eviction rate exceeds threshold", DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m"},
	{ID: "phpfpm_processes", ServiceType: "phpfpm", Name: "High active process usage", DefaultThreshold: 80, Unit: "%", Description: "Fires when active PHP-FPM processes exceed threshold percentage", DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m"},
}

func FindTemplateByID(id string) *AlertTemplate {
	for _, t := range AlertTemplates {
		if t.ID == id {
			return &t
		}
	}
	return nil
}

func TemplatesByServiceType(serviceType string) []AlertTemplate {
	var result []AlertTemplate
	for _, t := range AlertTemplates {
		if t.ServiceType == serviceType {
			result = append(result, t)
		}
	}
	return result
}
