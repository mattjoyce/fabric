package anthropic

import (
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/danielmiessler/fabric/internal/domain"
	"github.com/stretchr/testify/require"
)

// The anthropic and vertexai plugins set the sampling parameters through SetSamplingParams.
func TestSetSamplingParamsOmitsThemForModelsThatRejectThem(t *testing.T) {
	for _, model := range []string{"claude-opus-4-7@20260101", "claude-opus-5-5", "claude-sonnet-5-5@20260601"} {
		t.Run(model, func(t *testing.T) {
			for _, topP := range []float64{domain.DefaultTopP, 0.5} {
				var params anthropic.MessageNewParams
				SetSamplingParams(&params, &domain.ChatOptions{Model: model, Temperature: domain.DefaultTemperature, TopP: topP})

				require.False(t, params.Temperature.Valid())
				require.False(t, params.TopP.Valid())
			}
		})
	}
}

func TestSetSamplingParamsSetsOneOfThemForOtherModels(t *testing.T) {
	var params anthropic.MessageNewParams
	SetSamplingParams(&params, &domain.ChatOptions{Model: "claude-sonnet-4-6", Temperature: 0.3, TopP: domain.DefaultTopP})
	require.True(t, params.Temperature.Valid())
	require.False(t, params.TopP.Valid())

	params = anthropic.MessageNewParams{}
	SetSamplingParams(&params, &domain.ChatOptions{Model: "claude-sonnet-4-6", Temperature: 0.3, TopP: 0.5})
	require.True(t, params.TopP.Valid())
	require.False(t, params.Temperature.Valid())
}
