package handler

import (
	"testing"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/catalog"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
)

func TestMetricsEndpointForIntegrationFallsBackToCatalogForUnknownMode(t *testing.T) {
	svc := &domain.DiscoveredService{
		ServiceName: "nginx",
		MetricsMode: "unknown",
	}
	def, ok := catalog.Get("nginx")
	if !ok {
		t.Fatal("expected nginx catalog definition")
	}

	got := metricsEndpointForIntegration(svc, &def)

	if got != def.DefaultMetricsEndpoint {
		t.Fatalf("expected catalog default endpoint %q, got %q", def.DefaultMetricsEndpoint, got)
	}
}

func TestMetricsEndpointForIntegrationPrefersUserEndpointForUnknownMode(t *testing.T) {
	svc := &domain.DiscoveredService{
		ServiceName:         "nginx",
		MetricsMode:         "unknown",
		UserMetricsEndpoint: "http://127.0.0.1:9999/custom",
	}
	def, ok := catalog.Get("nginx")
	if !ok {
		t.Fatal("expected nginx catalog definition")
	}

	got := metricsEndpointForIntegration(svc, &def)

	if got != svc.UserMetricsEndpoint {
		t.Fatalf("expected user endpoint %q, got %q", svc.UserMetricsEndpoint, got)
	}
}

func TestMetricsEndpointForIntegrationPrefersUserEndpointForCatalogMode(t *testing.T) {
	svc := &domain.DiscoveredService{
		ServiceName:         "nginx",
		MetricsMode:         "catalog",
		UserMetricsEndpoint: "http://127.0.0.1:8081/nginx_status",
	}
	def, ok := catalog.Get("nginx")
	if !ok {
		t.Fatal("expected nginx catalog definition")
	}

	got := metricsEndpointForIntegration(svc, &def)

	if got != svc.UserMetricsEndpoint {
		t.Fatalf("expected user endpoint %q, got %q", svc.UserMetricsEndpoint, got)
	}
}
