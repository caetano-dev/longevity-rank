package rules

import (
	"fmt"
	
	"strings"
	"longevity-ranker/internal/models"
)

// VendorConfig holds specific rules for a website
type VendorConfig struct {
	// Words that, if found in the title, disqualify the product (e.g. "5-HTP", "Bundle")
	Blocklist []string
	
	// Manual overrides for tricky products where Regex fails
	// Key = Product Handle (URL slug), Value = Hardcoded attributes
	Overrides map[string]ProductSpec
}

type ProductSpec struct {
	ForceType string  // Force "Powder" or "Capsules"
	ForceMg   float64 // Force dosage (e.g. 500)
	ForceCount float64 // Force pill count (e.g. 30)
}

var Registry = map[string]VendorConfig{
	"Nutricost": {
		Blocklist: []string{"5-HTP", "Carnitine", "Caffeine", "Creatine", "Pre-Workout"},
	},
	"NMN Bio": {
		Blocklist: []string{"Bundle", "Endurance", "Book"},
		Overrides: map[string]ProductSpec{
			// The scraper misses dosage for this specific item? Hardcode it.
			"nmn-supplement-500mg-capsules-30-caps": {
				ForceType: "Capsules",
				ForceMg:   500,
				ForceCount: 30,
			},
		},
	},
	"Do Not Age": {
		Blocklist: []string{"Test", "Kit", "Consultation"},
	},
}

// ApplyRules runs the product through the vendor's specific constraints
func ApplyRules(vendorName string, p *models.Product) bool {
	config, exists := Registry[vendorName]
	if !exists {
		return true // No rules, keep product
	}

	// 1. Check Blocklist
	identity := strings.ToLower(p.Title + " " + p.Handle)
	for _, blocked := range config.Blocklist {
		if strings.Contains(identity, strings.ToLower(blocked)) {
			return false // Reject product
		}
	}
	
	// 2. Apply Overrides (Enrichment)
	// We inject the missing data into the Context so the Analyzer can find it
	if spec, ok := config.Overrides[p.Handle]; ok {
		// We artificially append the missing data to the Context string
		// The Analyzer scans Context, so it will pick this up!
		extraContext := ""
		if spec.ForceMg > 0 {
			extraContext += fmt.Sprintf(" %0.fmg", spec.ForceMg)
		}
		if spec.ForceCount > 0 {
			extraContext += fmt.Sprintf(" %0.f capsules", spec.ForceCount)
		}
		p.Context += extraContext
	}

	return true
}