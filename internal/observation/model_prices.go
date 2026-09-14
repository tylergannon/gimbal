package observation

import "strings"

// modelPrice is one model's base USD rate per million tokens. The generator
// owns its rows; this file owns the lookup and the pricing rule.
type modelPrice struct {
	provider    string
	id          string
	name        string
	releaseDate string
	lastUpdated string
	input       float64
	output      float64
	cacheRead   float64
	cacheWrite  float64
}

// modelPriceFor finds the exact model first. Harness effort suffixes name the
// same priced model, so one terminal suffix is removed for the second lookup.
func modelPriceFor(model string) (modelPrice, bool) {
	for _, price := range modelPrices {
		if price.id == model {
			return price, true
		}
	}
	for _, suffix := range []string{"-low", "-medium", "-high"} {
		if strings.HasSuffix(model, suffix) {
			model = strings.TrimSuffix(model, suffix)
			break
		}
	}
	for _, price := range modelPrices {
		if price.id == model {
			return price, true
		}
	}
	return modelPrice{}, false
}

// pricedCost calculates a model's base-price proxy. Reasoning tokens are
// billed at the output rate. Unknown models deliberately have no price.
func pricedCost(model string, tokens Tokens) (float64, bool) {
	price, ok := modelPriceFor(model)
	if !ok {
		return 0, false
	}
	return (tokens.Input*price.input +
		tokens.CacheRead*price.cacheRead +
		tokens.CacheWrite*price.cacheWrite +
		(tokens.Output+tokens.Reasoning)*price.output) / 1_000_000, true
}
