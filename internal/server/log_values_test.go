package restapi

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/danielmiessler/fabric/internal/core"
	"github.com/danielmiessler/fabric/internal/plugins"
	"github.com/danielmiessler/fabric/internal/plugins/ai"
	"github.com/danielmiessler/fabric/internal/plugins/db/fsdb"
	"github.com/danielmiessler/fabric/internal/tools"
	"github.com/gin-gonic/gin"
)

// captureLog returns what the standard logger writes until the test ends.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return &buf
}

// A client picks the language string. A line break in it must not start a
// line of its own in the server log.
func TestHandleChat_LogsLanguageWithoutLineBreaks(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	gin.SetMode(gin.TestMode)

	db := fsdb.NewDb(t.TempDir())
	vm := ai.NewVendorsManager()
	vm.AddVendors(&serverTestVendor{name: "TestVendor", models: []string{"test-model"}})
	registry := &core.PluginRegistry{
		Db:            db,
		VendorManager: vm,
		Defaults: &tools.Defaults{
			PluginBase:         &plugins.PluginBase{},
			Vendor:             &plugins.Setting{Value: "TestVendor"},
			Model:              &plugins.SetupQuestion{Setting: &plugins.Setting{Value: "test-model"}},
			ModelContextLength: &plugins.SetupQuestion{Setting: &plugins.Setting{Value: "0"}},
		},
	}
	router := gin.New()
	NewChatHandler(router, registry, db)

	logged := captureLog(t)

	body := `{"language":"en\n2026/01/01 00:00:00 FORGED admin login","prompts":[]}`
	req := httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := &closeNotifierRecorder{
		ResponseRecorder: httptest.NewRecorder(),
		closeCh:          make(chan bool, 1),
	}
	router.ServeHTTP(resp, req)

	for line := range strings.SplitSeq(strings.TrimRight(logged.String(), "\n"), "\n") {
		if strings.Contains(line, "FORGED") && !strings.Contains(line, "Received chat request") {
			t.Fatalf("the language started a log line of its own: %q", line)
		}
	}
	if !strings.Contains(logged.String(), "Received chat request") {
		t.Fatalf("expected the request line in the log, got %q", logged.String())
	}
}

// The body of an upstream error can hold line breaks and a lot of text. The
// log keeps it on one line and cuts it.
func TestOllamaChat_LogsUpstreamErrorBodyOnOneLine(t *testing.T) {
	gin.SetMode(gin.TestMode)

	longTail := strings.Repeat("x", 4*maxLoggedBodyBytes)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("first\n2026/01/01 00:00:00 FORGED admin login\n" + longTail))
	}))
	defer upstream.Close()

	logged := captureLog(t)

	r := gin.New()
	conv := APIConvert{addr: &upstream.URL}
	r.POST("/api/chat", conv.ollamaChat)
	req := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(`{"model":"test:latest","messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(httptest.NewRecorder(), req)

	for line := range strings.SplitSeq(strings.TrimRight(logged.String(), "\n"), "\n") {
		if strings.Contains(line, "FORGED") && !strings.Contains(line, "first") {
			t.Fatalf("the upstream body started a log line of its own: %q", line)
		}
		if len(line) > 2*maxLoggedBodyBytes {
			t.Fatalf("the upstream body was not cut: log line has %d bytes", len(line))
		}
	}
}
