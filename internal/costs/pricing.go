package costs

import "strings"

const perMillion = 1_000_000.0

// EstimateAnthropicUSD returns estimated USD cost for Anthropic models.
// Returns ok=false when no known fallback pricing exists for the model.
func EstimateAnthropicUSD(model string, inputTokens, outputTokens int) (usd float64, ok bool) {
	modelName := strings.ToLower(strings.TrimSpace(model))

	var inputPerMillion float64
	var outputPerMillion float64

	switch {
	case strings.Contains(modelName, "haiku"):
		inputPerMillion = 0.80
		outputPerMillion = 4.00
	case strings.Contains(modelName, "sonnet"):
		inputPerMillion = 3.00
		outputPerMillion = 15.00
	case strings.Contains(modelName, "opus"):
		inputPerMillion = 15.00
		outputPerMillion = 75.00
	default:
		return 0, false
	}

	inputCost := (float64(inputTokens) / perMillion) * inputPerMillion
	outputCost := (float64(outputTokens) / perMillion) * outputPerMillion
	return inputCost + outputCost, true
}

// EstimateUSD returns fallback estimated USD cost for providers that require
// local pricing.
func EstimateUSD(providerName, model string, inputTokens, outputTokens int) (usd float64, ok bool) {
	switch strings.ToLower(strings.TrimSpace(providerName)) {
	case "anthropic":
		return EstimateAnthropicUSD(model, inputTokens, outputTokens)
	default:
		return 0, false
	}
}
