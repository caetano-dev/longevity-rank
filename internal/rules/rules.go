package rules

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"longevity-ranker/internal/models"
)

// ProductSpec defines immutable mathematical truths about a product that the
// regex engine cannot reliably extract. When present, these values bypass
// regex entirely — they are not hints, they are overrides.
type ProductSpec struct {
	ForceType              string             `json:"forceType,omitempty"`
	ForceActiveGrams       float64            `json:"forceActiveGrams,omitempty"`
	ForceServingMg         float64            `json:"forceServingMg,omitempty"`
	VariantOverrides       map[string]float64 `json:"variantOverrides,omitempty"`
	VariantGrossOverrides  map[string]float64 `json:"variantGrossOverrides,omitempty"`
}

type VendorConfig struct {
	Blocklist                  []string               `json:"blocklist"`
	VariantBlocklist           []string               `json:"variantBlocklist,omitempty"`
	Overrides                  map[string]ProductSpec  `json:"overrides"`
	GlobalSubscriptionDiscount float64                 `json:"globalSubscriptionDiscount,omitempty"`
}

// Registry holds the loaded rules
var Registry map[string]VendorConfig

// LoadRules reads the JSON configuration from disk
func LoadRules(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("could not open rules file: %v", err)
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("could not read rules file: %v", err)
	}

	if err := json.Unmarshal(bytes, &Registry); err != nil {
		return fmt.Errorf("could not parse rules file: %v", err)
	}

	return nil
}

// ApplyRules evaluates the vendor blocklist against the product. Returns false
// if the product is blocked, true if it is allowed. This function performs NO
// data enrichment — overrides are consumed directly by the analyzer.
func ApplyRules(vendorName string, p *models.Product) bool {
	if Registry == nil {
		return true // No rules loaded, safe default
	}

	config, exists := Registry[vendorName]
	if !exists {
		return true
	}

	// Check Blocklist
	identity := strings.ToLower(p.Title + " " + p.Handle + " " + p.Context)
	for _, blocked := range config.Blocklist {
		if strings.Contains(identity, strings.ToLower(blocked)) {
			return false // Reject product
		}
	}

	return true
}