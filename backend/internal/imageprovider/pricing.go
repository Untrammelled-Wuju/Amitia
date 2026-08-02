package imageprovider

type PriceUnit string

const (
	PriceUnitPerImage   PriceUnit = "per_image"
	PriceUnitPerRequest PriceUnit = "per_request"
	PriceUnitPerPixel   PriceUnit = "per_pixel"
)

type PriceEntry struct {
	Provider    string         `json:"provider"`
	Model       string         `json:"model"`
	Mode        GenerationMode `json:"mode"`
	Width       int            `json:"width"`
	Height      int            `json:"height"`
	OutputCount int            `json:"outputCount"`
	Unit        PriceUnit      `json:"unit"`
	Amount      *float64       `json:"amount"`
	Currency    string         `json:"currency"`
	EffectiveAt string         `json:"effectiveAt"`
	SourceLabel string         `json:"sourceLabel"`
	Version     string         `json:"version"`
}

type PricingCatalog struct {
	entries []PriceEntry
}

func NewPricingCatalog() *PricingCatalog {
	return &PricingCatalog{}
}

func (c *PricingCatalog) Add(entry PriceEntry) {
	c.entries = append(c.entries, entry)
}

func (c *PricingCatalog) Match(provider, model string, mode GenerationMode, width, height, outputCount int) (PriceEntry, bool) {
	var best PriceEntry
	found := false
	for _, e := range c.entries {
		if e.Provider != provider {
			continue
		}
		if e.Model != "" && e.Model != model {
			continue
		}
		if e.Mode != "" && e.Mode != mode {
			continue
		}
		if e.Width > 0 && e.Width != width {
			continue
		}
		if e.Height > 0 && e.Height != height {
			continue
		}
		if e.OutputCount > 0 && e.OutputCount != outputCount {
			continue
		}
		if !found {
			best = e
			found = true
			continue
		}
		if e.Model != "" && best.Model == "" {
			best = e
		}
		if e.Width > 0 && best.Width == 0 {
			best = e
		}
		if e.OutputCount > 0 && best.OutputCount == 0 {
			best = e
		}
	}
	return best, found
}

type CostEstimate struct {
	PriceKnown     bool     `json:"priceKnown"`
	Currency       string   `json:"currency"`
	EstimatedMin   *float64 `json:"estimatedMin"`
	EstimatedMax   *float64 `json:"estimatedMax"`
	PricingSource  string   `json:"pricingSource"`
	PricingVersion string   `json:"pricingVersion"`
	Warnings       []string `json:"warnings"`
}

func (c *PricingCatalog) Estimate(provider, model string, mode GenerationMode, width, height, outputCount, primaryRequests, maxCalls int) CostEstimate {
	entry, found := c.Match(provider, model, mode, width, height, outputCount)
	if !found || entry.Amount == nil {
		return CostEstimate{
			PriceKnown:    false,
			PricingSource: "unknown",
			Warnings:      []string{"price_not_found_in_catalog"},
		}
	}
	minCost := *entry.Amount * float64(primaryRequests)
	maxCost := *entry.Amount * float64(maxCalls)
	return CostEstimate{
		PriceKnown:     true,
		Currency:       entry.Currency,
		EstimatedMin:   &minCost,
		EstimatedMax:   &maxCost,
		PricingSource:  entry.SourceLabel,
		PricingVersion: entry.Version,
	}
}
