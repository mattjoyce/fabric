package gemini

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielmiessler/fabric/internal/chat"
	"github.com/danielmiessler/fabric/internal/domain"
	"github.com/stretchr/testify/require"
)

var (
	testMsgs = []*chat.ChatCompletionMessage{{Role: chat.ChatMessageRoleUser, Content: "hi"}}
	testOpts = &domain.ChatOptions{Model: "gemini-3.8-flash"}
)

// newTestClient returns a client whose requests go to a test server that sends body.
func newTestClient(t *testing.T, contentType, body string) *Client {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		_, _ = fmt.Fprint(w, body)
	}))
	t.Cleanup(server.Close)
	t.Setenv("GOOGLE_GEMINI_BASE_URL", server.URL)

	client := NewClient()
	client.ApiKey.Value = "test-key"
	return client
}

func streamGemini(t *testing.T, dataLines ...string) (updates []domain.StreamUpdate, err error) {
	t.Helper()

	body := ""
	for _, line := range dataLines {
		body += "data: " + line + "\n\n"
	}
	channel := make(chan domain.StreamUpdate, 16)
	err = newTestClient(t, "text/event-stream", body).SendStream(context.Background(), testMsgs, testOpts, channel)
	for update := range channel {
		updates = append(updates, update)
	}
	return updates, err
}

func TestSendStreamReturnsErrorWhenThePromptIsBlocked(t *testing.T) {
	_, err := streamGemini(t,
		`{"promptFeedback":{"blockReason":"PROHIBITED_CONTENT"},"usageMetadata":{"promptTokenCount":3,"totalTokenCount":3}}`,
	)

	require.Error(t, err)
	require.Contains(t, err.Error(), "PROHIBITED_CONTENT")
}

func TestSendStreamReturnsErrorWhenTheAnswerIsStoppedBySafety(t *testing.T) {
	updates, err := streamGemini(t,
		`{"candidates":[{"content":{"role":"model","parts":[{"text":"Hel"}]}}]}`,
		`{"candidates":[{"finishReason":"SAFETY"}],"usageMetadata":{"promptTokenCount":3,"candidatesTokenCount":1,"totalTokenCount":4}}`,
	)

	require.Error(t, err)
	require.Contains(t, err.Error(), "SAFETY")
	require.Equal(t, "Hel", updates[0].Content)
}

func TestSendStreamNormalAndMaxTokensStopsAreNotErrors(t *testing.T) {
	for _, reason := range []string{"STOP", "MAX_TOKENS"} {
		t.Run(reason, func(t *testing.T) {
			updates, err := streamGemini(t,
				fmt.Sprintf(`{"candidates":[{"content":{"role":"model","parts":[{"text":"Hello"}]},"finishReason":%q}]}`, reason),
			)

			require.NoError(t, err)
			require.Equal(t, "Hello", updates[0].Content)
		})
	}
}

func TestSendReturnsErrorWhenTheAnswerIsStoppedBySafety(t *testing.T) {
	_, err := newTestClient(t, "application/json", `{"candidates":[{"finishReason":"SAFETY"}]}`).
		Send(context.Background(), testMsgs, testOpts)

	require.Error(t, err)
	require.Contains(t, err.Error(), "SAFETY")
}

func TestSendReturnsTheAnswerOfANormalResponse(t *testing.T) {
	answer, err := newTestClient(t, "application/json", `{"candidates":[{"content":{"role":"model","parts":[{"text":"Hello"}]},"finishReason":"STOP"}]}`).
		Send(context.Background(), testMsgs, testOpts)

	require.NoError(t, err)
	require.Equal(t, "Hello", answer)
}
