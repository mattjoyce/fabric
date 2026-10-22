package restapi

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/danielmiessler/fabric/internal/core"
	"github.com/danielmiessler/fabric/internal/plugins/db/fsdb"
	"github.com/gin-gonic/gin"
)

// Each response, including the ones that the API key or loopback checks
// reject, tells the browser not to sniff the content type and not to store
// the response. A handler that sets its own Cache-Control keeps it.
func TestServeEngineSecurityHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("OPENAI_API_KEY", "")
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "contexts"), 0o700); err != nil {
		t.Fatal(err)
	}
	registry := &core.PluginRegistry{Db: fsdb.NewDb(dir)}

	cases := []struct {
		name   string
		r      *gin.Engine
		host   string
		path   string
		status int
	}{
		{"unauthorized", newServeEngine(registry, "secret", nil), "localhost", "/patterns/names", http.StatusUnauthorized},
		{"foreign host", newServeEngine(registry, "", nil), "evil.example", "/patterns/names", http.StatusForbidden},
		{"authorized", newServeEngine(registry, "", nil), "localhost", "/contexts/names", http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			req.Host = tc.host
			rec := httptest.NewRecorder()
			tc.r.ServeHTTP(rec, req)

			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d", rec.Code, tc.status)
			}
			if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
				t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
			}
			if got := rec.Header().Get("Cache-Control"); got != "no-store" {
				t.Errorf("Cache-Control = %q, want no-store", got)
			}
		})
	}
}

func TestSecurityHeadersMiddleware_HandlerCacheControlWins(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SecurityHeadersMiddleware())
	r.GET("/stream", func(c *gin.Context) {
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.String(http.StatusOK, "ok")
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/stream", nil))
	if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control = %q, want the handler's no-cache", got)
	}
}
