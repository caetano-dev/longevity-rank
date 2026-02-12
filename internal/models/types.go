package models

type Vendor struct {
	Name string
	URL  string
	Type string
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
	Price string `json:"price"`
	Title string `json:"title"`
	Available bool `json:"available"`
}

type Analysis struct {
	Vendor      string
	Name        string
	Price       float64
	TotalGrams  float64
	CostPerGram float64
	EffectiveCost float64
	Type        string 
	ImageURL    string
}