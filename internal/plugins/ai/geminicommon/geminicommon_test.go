package geminicommon

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/genai"
)

func TestExtractTextSkipsThoughtParts(t *testing.T) {
	response := &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{{
			Content: &genai.Content{Parts: []*genai.Part{
				{Text: "**Planning the answer**", Thought: true},
				{Text: "The answer is 2."},
			}},
		}},
	}

	assert.Equal(t, "The answer is 2.", ExtractText(response))
	assert.Equal(t, "The answer is 2.", ExtractTextWithCitations(response))
}

func TestOutputTokenCountIncludesThoughts(t *testing.T) {
	usage := &genai.GenerateContentResponseUsageMetadata{
		PromptTokenCount:     10,
		CandidatesTokenCount: 20,
		ThoughtsTokenCount:   70,
		TotalTokenCount:      100,
	}

	assert.Equal(t, 90, OutputTokenCount(usage))
}

func TestBlockedError(t *testing.T) {
	tests := []struct {
		name     string
		response *genai.GenerateContentResponse
		wantErr  string
	}{
		{"nil response", nil, ""},
		{"ordinary answer", &genai.GenerateContentResponse{Candidates: []*genai.Candidate{{FinishReason: genai.FinishReasonStop}}}, ""},
		{"cut off by max tokens", &genai.GenerateContentResponse{Candidates: []*genai.Candidate{{FinishReason: genai.FinishReasonMaxTokens}}}, ""},
		{
			"blocked prompt",
			&genai.GenerateContentResponse{PromptFeedback: &genai.GenerateContentResponsePromptFeedback{BlockReason: genai.BlockedReasonSafety}},
			"the prompt was blocked: SAFETY",
		},
		{
			"blocked prompt with message",
			&genai.GenerateContentResponse{PromptFeedback: &genai.GenerateContentResponsePromptFeedback{BlockReason: genai.BlockedReasonOther, BlockReasonMessage: "not allowed"}},
			"the prompt was blocked: OTHER (not allowed)",
		},
		{
			"stopped by safety",
			&genai.GenerateContentResponse{Candidates: []*genai.Candidate{{FinishReason: genai.FinishReasonSafety, FinishMessage: "unsafe"}}},
			"the response was stopped: SAFETY (unsafe)",
		},
		{
			"stopped by prohibited image content",
			&genai.GenerateContentResponse{Candidates: []*genai.Candidate{{FinishReason: genai.FinishReasonImageProhibitedContent}}},
			"the response was stopped: IMAGE_PROHIBITED_CONTENT",
		},
		{
			"stopped by image recitation",
			&genai.GenerateContentResponse{Candidates: []*genai.Candidate{{FinishReason: genai.FinishReasonImageRecitation}}},
			"the response was stopped: IMAGE_RECITATION",
		},
		{
			"stopped by a blocklist",
			&genai.GenerateContentResponse{Candidates: []*genai.Candidate{nil, {FinishReason: genai.FinishReasonBlocklist}}},
			"the response was stopped: BLOCKLIST",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := BlockedError(tt.response)
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tt.wantErr)
			}
		})
	}
}
