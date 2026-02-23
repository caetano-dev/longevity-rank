package parser

import (
	"fmt"
	"math"
	"strings"

	"longevity-ranker/internal/models"
)

// AuditResult describes a product that passes interest/blocklist filters but
// lacks enough data for the analyzer to compute activeGrams. It reports what
// data we DO have and what is MISSING so the operator can add an override
// in data/vendor_rules.json.
type AuditResult struct {
	Vendor     string
	Title      string
	Handle     string
	BestPrice  float64
	VariantCt  int
	MgFound    bool
	MgValue    float64
	CountFound bool
	CountValue float64
	GramsFound bool
	GramsValue float64
	KgFound    bool
	KgValue    float64
	Missing    []string
}

// AuditProduct runs the same extraction pipeline as AnalyzeProduct but never
// silently discards products. If the product passes the supplement keyword
// gate but the analyzer would return nil (no computable activeGrams), this
// function returns an *AuditResult describing the gap. If the product is
// fully analyzable (AnalyzeProduct would succeed), it returns nil — no gap.
//
// This function assumes ApplyRules has already been called (blocklist filtering).
func (a *Analyzer) AuditProduct(vendorName string, p models.Product) *AuditResult {
	if len(p.Variants) == 0 {
		return &AuditResult{
			Vendor:  vendorName,
			Title:   p.Title,
			Handle:  p.Handle,
			Missing: []string{"no variants at all"},
		}
	}

	// Supplement keyword gate (same as AnalyzeProduct)
	identity := strings.ToLower(p.Title + " " + p.Context + " " + p.Handle)
	if !a.matchesSupplement(identity) {
		return nil // Not a supplement we track — not a gap, just irrelevant
	}

	// Check if a catalog override already provides total grams
	if a.Rules != nil {
		if config, exists := a.Rules[vendorName]; exists {
			if spec, hasOverride := config.Overrides[p.Handle]; hasOverride && spec.ForceActiveGrams > 0 {
				if a.AnalyzeProduct(vendorName, p) != nil {
					return nil
				}
			}
		}
	}

	// Check if AnalyzeProduct already succeeds via regex path
	if a.AnalyzeProduct(vendorName, p) != nil {
		return nil
	}

	// The product IS interesting but the analyzer rejected it. Diagnose.
	result := &AuditResult{
		Vendor:    vendorName,
		Title:     p.Title,
		Handle:    p.Handle,
		BestPrice: math.MaxFloat64,
	}

	availableCount := 0
	for _, v := range p.Variants {
		if !v.Available {
			continue
		}
		price, ok := extractFloat(rePriceFloat, v.Price)
		if !ok {
			continue
		}
		availableCount++
		if price < result.BestPrice {
			result.BestPrice = price
		}
	}
	result.VariantCt = availableCount

	if availableCount == 0 {
		result.BestPrice = 0
		result.Missing = append(result.Missing, "no available variants with a valid price")
		return result
	}

	// Build search strings for probing
	broadSearch := p.Title + " " + p.Context + " " + strings.ReplaceAll(p.Handle, "-", " ") + " " + p.BodyHTML
	cleanSearch := p.Title
	variantSearch := ""
	for _, v := range p.Variants {
		broadSearch += " " + v.Title
		cleanSearch += " " + v.Title
		variantSearch += " " + v.Title
	}

	// Probe: explicit grams (clean first, then broad fallback)
	if g, ok := extractFloatFrom(reGrams, cleanSearch, broadSearch); ok {
		result.GramsFound = true
		result.GramsValue = g
	}

	// Probe: kg
	if kg, ok := extractFloat(reKg, cleanSearch); ok {
		result.KgFound = true
		result.KgValue = kg
	}

	// Probe: mg
	if mg, ok := extractFloat(reMg, broadSearch); ok {
		result.MgFound = true
		result.MgValue = mg
	}

	// Probe: count
	if c, ok := extractFloatFrom(reCount, variantSearch, cleanSearch, broadSearch); ok {
		result.CountFound = true
		result.CountValue = c
	}

	// Diagnose what's missing
	hasPowderMass := result.GramsFound || result.KgFound
	hasCapsuleMass := result.MgFound && result.CountFound

	if !hasPowderMass && !hasCapsuleMass {
		if !result.MgFound {
			result.Missing = append(result.Missing, "mg per serving (forceServingMg)")
		}
		if !result.CountFound {
			result.Missing = append(result.Missing, "capsule/tablet count")
		}
		if !result.GramsFound && !result.KgFound {
			result.Missing = append(result.Missing, "active grams (forceActiveGrams)")
		}
	} else {
		result.Missing = append(result.Missing, "data was partially found but activeGrams still computed to 0 (check overrides)")
	}

	return result
}

// FormatAuditReport produces a human-readable multi-line string from a slice
// of AuditResults, suitable for printing to stdout.
func FormatAuditReport(results []AuditResult) string {
	if len(results) == 0 {
		return "✅ No gaps detected. All interesting products have enough data for analysis."
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("\n🔍 AUDIT: %d product(s) need manual overrides in data/vendor_rules.json\n", len(results)))
	b.WriteString(strings.Repeat("─", 80) + "\n")

	// Group by vendor, preserving insertion order
	grouped := make(map[string][]AuditResult)
	var vendorOrder []string
	for _, r := range results {
		if _, seen := grouped[r.Vendor]; !seen {
			vendorOrder = append(vendorOrder, r.Vendor)
		}
		grouped[r.Vendor] = append(grouped[r.Vendor], r)
	}

	for _, vendor := range vendorOrder {
		items := grouped[vendor]
		b.WriteString(fmt.Sprintf("\n📦 %s (%d item(s))\n", vendor, len(items)))
		for _, r := range items {
			b.WriteString(fmt.Sprintf("  ├─ Product: %s\n", r.Title))
			b.WriteString(fmt.Sprintf("  │  Handle:  %s\n", r.Handle))
			if r.VariantCt > 0 {
				b.WriteString(fmt.Sprintf("  │  Variants: %d available, best price: $%.2f\n", r.VariantCt, r.BestPrice))
			} else {
				b.WriteString("  │  Variants: none available\n")
			}

			// Show what we DO have
			var found []string
			if r.MgFound {
				found = append(found, fmt.Sprintf("mg=%.0f", r.MgValue))
			}
			if r.CountFound {
				found = append(found, fmt.Sprintf("count=%.0f", r.CountValue))
			}
			if r.GramsFound {
				found = append(found, fmt.Sprintf("grams=%.1f", r.GramsValue))
			}
			if r.KgFound {
				found = append(found, fmt.Sprintf("kg=%.2f", r.KgValue))
			}
			if len(found) > 0 {
				b.WriteString(fmt.Sprintf("  │  Found:   %s\n", strings.Join(found, ", ")))
			} else {
				b.WriteString("  │  Found:   (nothing extractable)\n")
			}

			// Show what's MISSING
			b.WriteString(fmt.Sprintf("  │  Missing: %s\n", strings.Join(r.Missing, "; ")))

			// Suggest override snippet
			b.WriteString("  │  Suggested override:\n")
			b.WriteString(fmt.Sprintf("  │    \"%s\": {\n", r.Handle))
			b.WriteString("  │      \"forceType\": \"Capsules\",\n")

			switch {
			case r.MgFound && r.CountFound:
				activeGrams := r.MgValue * r.CountValue / 1000.0
				b.WriteString(fmt.Sprintf("  │      \"forceActiveGrams\": %.1f,\n", activeGrams))
				b.WriteString(fmt.Sprintf("  │      \"forceServingMg\": %.0f\n", r.MgValue))
			case r.GramsFound:
				b.WriteString(fmt.Sprintf("  │      \"forceActiveGrams\": %.1f,\n", r.GramsValue))
				b.WriteString("  │      \"forceServingMg\": ???\n")
			case r.KgFound:
				b.WriteString(fmt.Sprintf("  │      \"forceActiveGrams\": %.1f,\n", r.KgValue*1000))
				b.WriteString("  │      \"forceServingMg\": ???\n")
			default:
				b.WriteString("  │      \"forceActiveGrams\": ???,\n")
				if r.MgFound {
					b.WriteString(fmt.Sprintf("  │      \"forceServingMg\": %.0f\n", r.MgValue))
				} else {
					b.WriteString("  │      \"forceServingMg\": ???\n")
				}
			}
			b.WriteString("  │    }\n")
			b.WriteString("  │\n")
		}
	}
	b.WriteString(strings.Repeat("─", 80) + "\n")
	return b.String()
}