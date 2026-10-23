package restapi

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danielmiessler/fabric/internal/core"
	"github.com/danielmiessler/fabric/internal/plugins/db/fsdb"
	"github.com/gin-gonic/gin"
)

func TestAuditLog(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var logs bytes.Buffer
	defer slog.SetDefault(slog.Default())
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))

	const key = "synthetic-api-key-0123456789"
	r := newServeEngine(&core.PluginRegistry{Db: fsdb.NewDb(t.TempDir())}, key, nil)
	send := func(method, path, apiKey, body string) {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set(APIKeyHeader, apiKey)
		r.ServeHTTP(httptest.NewRecorder(), req)
	}
	send(http.MethodPost, "/patterns/audited", key, "synthetic pattern body")
	send(http.MethodGet, "/patterns/audited", key, "")
	send(http.MethodPost, "/patterns/audited", "wrong-key", "")

	got := logs.String()
	for _, want := range []string{
		`msg="audit: change" method=POST path=/patterns/audited status=200`,
		`msg="API key wrong"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("log does not contain %s:\n%s", want, got)
		}
	}
	if n := strings.Count(got, "audit: change"); n != 1 {
		t.Errorf("got %d change events, want 1 (no event for GET or a rejected key):\n%s", n, got)
	}
	if strings.Contains(got, key) || strings.Contains(got, "wrong-key") || strings.Contains(got, "synthetic pattern body") {
		t.Errorf("log contains a key or a body:\n%s", got)
	}
}
