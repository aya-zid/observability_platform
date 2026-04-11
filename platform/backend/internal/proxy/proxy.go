// internal/proxy/proxy.go
package proxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/labstack/echo/v4"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/cache"
	custommiddleware "gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/middleware"
)

// ProxyConfig defines where to forward requests and what headers to inject.
type ProxyConfig struct {
	TargetURL     string
	RewritePath   func(tenantID uint, tenantUUID string) string
	InjectHeaders func(tenantID uint, tenantUUID string) map[string]string
}

func NewReverseProxy(cfg ProxyConfig) echo.HandlerFunc {
	target, err := url.Parse(cfg.TargetURL)
	if err != nil {
		panic(fmt.Sprintf("proxy: invalid target URL %q: %v", cfg.TargetURL, err))
	}

	return func(c echo.Context) error {
		tenant := getTenant(c)
		if tenant == nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
		}

		p := &httputil.ReverseProxy{
			Director: func(req *http.Request) {
				req.URL.Scheme = target.Scheme
				req.URL.Host = target.Host
				req.URL.Path = cfg.RewritePath(tenant.ID, tenant.UUID)
				req.Header.Del("X-Forwarded-For")
				req.Header.Del("X-Real-Ip")
				if cfg.InjectHeaders != nil {
					for k, v := range cfg.InjectHeaders(tenant.ID, tenant.UUID) {
						req.Header.Set(k, v)
					}
				}
				req.Host = target.Host
			},
			FlushInterval: -1,
			ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
				c.Logger().Errorf("[proxy] upstream error for tenant %s: %v", tenant.UUID, err)
				w.WriteHeader(http.StatusBadGateway)
			},
		}

		p.ServeHTTP(c.Response().Writer, c.Request())
		return nil
	}
}

// getTenant reads the *cache.CachedTenant set by IngestMiddleware.
func getTenant(c echo.Context) *cache.CachedTenant {
	v := c.Get(custommiddleware.TenantContextKey)
	if v == nil {
		return nil
	}
	t, ok := v.(*cache.CachedTenant)
	if !ok {
		c.Logger().Errorf("[proxy] context type mismatch: expected *cache.CachedTenant, got %T", v)
		return nil
	}
	return t
}
