package restapi

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/danielmiessler/fabric/internal/chat"
	"github.com/danielmiessler/fabric/internal/core"
	"github.com/danielmiessler/fabric/internal/domain"
	debuglog "github.com/danielmiessler/fabric/internal/log"
	"github.com/danielmiessler/fabric/internal/plugins"
	"github.com/danielmiessler/fabric/internal/plugins/ai"
	"github.com/danielmiessler/fabric/internal/plugins/db/fsdb"
	"github.com/danielmiessler/fabric/internal/tools"
	"github.com/gin-gonic/gin"
	"github.com/openai/openai-go/v3"
)

const privateText = "SYNTHETIC-PRIVATE-PATTERN-TEXT"

// failingVendor fails the way a provider does: it can send an error update
// and it returns an error, and both repeat the prompt.
type failingVendor struct {
	serverTestVendor
	update bool
}

func (m *failingVendor) SendStream(_ context.Context, _ []*chat.ChatCompletionMessage, _ *domain.ChatOptions, ch chan domain.StreamUpdate) error {
	defer close(ch)
	if m.update {
		ch <- domain.StreamUpdate{Type: domain.StreamTypeError, Content: "stream failed: " + privateText}
	}
	return fmt.Errorf("%w: prompt %q is bad (invalid_prompt)", &openai.Error{StatusCode: 400, Code: "invalid_prompt"}, privateText)
}

func TestHandleChatRedactsProviderErrors(t *testing.T) {
	for _, update := range []bool{false, true} {
		t.Run(fmt.Sprintf("update=%v", update), func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			gin.SetMode(gin.TestMode)
			var logs bytes.Buffer
			log.SetOutput(&logs)
			t.Cleanup(func() { log.SetOutput(os.Stderr) })

			db := fsdb.NewDb(t.TempDir())
			vm := ai.NewVendorsManager()
			vm.AddVendors(&failingVendor{serverTestVendor{name: "TestVendor", models: []string{"test-model"}}, update})
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

			body := `{"prompts":[{"userInput":"hello","vendor":"TestVendor","model":"test-model"}]}`
			req := httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp := &closeNotifierRecorder{ResponseRecorder: httptest.NewRecorder(), closeCh: make(chan bool, 1)}
			router.ServeHTTP(resp, req)

			got := resp.Body.String()
			if !strings.Contains(got, `"type":"error"`) {
				t.Fatalf("expected an error event, got %q", got)
			}
			if !update && !strings.Contains(got, "provider error: status 400, code invalid_prompt") {
				t.Fatalf("expected the status and the code, got %q", got)
			}
			if strings.Contains(got, privateText) || strings.Contains(logs.String(), privateText) {
				t.Fatalf("provider text leaked.\nbody: %q\nlog: %q", got, logs.String())
			}
		})
	}
}

func TestPublicError(t *testing.T) {
	plain := errors.New("pattern needs variable x")
	if got := publicError(plain); got != plain.Error() {
		t.Errorf("a Fabric error must not change: got %q", got)
	}
	provider := &domain.ProviderError{Err: errors.New(privateText)}
	if got := publicError(provider); got != "provider error" {
		t.Errorf("got %q, want %q", got, "provider error")
	}

	debuglog.SetLevel(debuglog.Basic)
	t.Cleanup(func() { debuglog.SetLevel(debuglog.Off) })
	if got := publicError(provider); got != privateText {
		t.Errorf("with --debug, got %q, want the full text", got)
	}
}
