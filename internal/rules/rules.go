package rules

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"longevity-ranker/internal/models"
)

// ProductSpec defines immutable mathematical truths about a product that the
// regex engine cannot reliably extract. When present, these values bypass
// regex entirely — they are not hints, they are overrides.
type ProductSpec struct {
	ForceType             string             `json:"forceType,omitempty"`
	ForceActiveGrams      float64            `json:"forceActiveGrams,omitempty"`
	ForceServingMg        float64            `json:"forceServingMg,omitempty"`
	VariantOverrides      map[string]float64 `json:"variantOverrides,omitempty"`
	VariantGrossOverrides map[string]float64 `json:"variantGrossOverrides,omitempty"`
}

// VendorConfig holds blocklist and override configuration for a single vendor.
type VendorConfig struct {
	Blocklist                  []string              `json:"blocklist"`
	VariantBlocklist           []string              `json:"variantBlocklist,omitempty"`
	Overrides                  map[string]ProductSpec `json:"overrides"`
	GlobalSubscriptionDiscount float64               `json:"globalSubscriptionDiscount,omitempty"`
}

// Registry is a map from vendor name to its configuration.
type Registry = map[string]VendorConfig

// LoadRules reads the JSON configuration from disk and returns the registry.
// The caller owns the returned map — there is no global mutable state.
func LoadRules(path string) (Registry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read rules file: %v", err)
	}

	var reg Registry
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("could not parse rules file: %v", err)
	}

	return reg, nil
}

// ApplyRules evaluates the vendor blocklist against the product. Returns false
// if the product is blocked, true if it is allowed. This function performs NO
// data enrichment — overrides are consumed directly by the analyzer.
func ApplyRules(reg Registry, vendorName string, p *models.Product) bool {
	if reg == nil {
		return true
	}

	config, exists := reg[vendorName]
	if !exists {
		return true
	}

	identity := strings.ToLower(p.Title + " " + p.Handle + " " + p.Context)
	for _, blocked := range config.Blocklist {
		if strings.Contains(identity, strings.ToLower(blocked)) {
			return false
		}
	}

	return true
}