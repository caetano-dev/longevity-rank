package models

// Vendor represents a website we want to scrape
type Vendor struct {
	Name string
	URL  string
}

// Shopify Response Structures
type ShopifyResponse struct {
	Products []Product `json:"products"`
}

type Product struct {
	ID       int64     `json:"id"`
	Title    string    `json:"title"`
	Handle   string    `json:"handle"` // URL slug (useful for finding hidden info)
	BodyHTML string    `json:"body_html"` // Description (fallback for dosage)
	Variants []Variant `json:"variants"`
}

type Variant struct {
	Price string `json:"price"`
}

// Analysis is our final calculated result
type Analysis struct {
	Vendor      string
	Name        string
	Price       float64
	TotalGrams  float64
	CostPerGram float64
	Type        string // Single, Bundle, Powder
}