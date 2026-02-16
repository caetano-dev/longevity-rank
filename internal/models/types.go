package models

type Vendor struct {
	Name       string
	URL        string
	Type       string
	Cloudflare bool
}

type Product struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	Context  string    `json:"context"`
	Handle   string    `json:"handle"`
	BodyHTML string    `json:"body_html"`
	ImageURL string    `json:"image_url"`
	Variants []Variant `json:"variants"`
}

type Variant struct {
	Price     string `json:"price"`
	Title     string `json:"title"`
	Available bool   `json:"available"`
}

type Analysis struct {
	Vendor          string  `json:"vendor"`
	Name            string  `json:"name"`
	Handle          string  `json:"handle"`
	Price           float64 `json:"price"`
	TotalGrams      float64 `json:"total_grams"`
	CostPerGram     float64 `json:"cost_per_gram"`
	EffectiveCost   float64 `json:"effective_cost"`
	Multiplier      float64 `json:"multiplier"`
	MultiplierLabel string  `json:"multiplier_label"`
	Type            string  `json:"type"`
	ImageURL        string  `json:"image_url"`
	IsSubscription  bool    `json:"is_subscription"`
	NeedsReview     bool    `json:"needs_review"`
	ReviewReason    string  `json:"review_reason,omitempty"`
}