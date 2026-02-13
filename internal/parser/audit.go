package parser

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"longevity-ranker/internal/models"
)

// AuditResult describes a product that passes interest/blocklist filters but
// lacks enough data for the analyzer to compute totalGrams. It reports what
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
// gate but the analyzer would return nil (no computable totalGrams), this
// function returns an *AuditResult describing the gap. If the product is
// fully analyzable (AnalyzeProduct would succeed), it returns nil — no gap.
func AuditProduct(vendorName string, p models.Product) *AuditResult {
	if len(p.Variants) == 0 {
		return &AuditResult{
			Vendor:  vendorName,
			Title:   p.Title,
			Handle:  p.Handle,
			Missing: []string{"no variants at all"},
		}
	}

	// --- Supplement keyword gate (same as AnalyzeProduct) ---
	identityString := strings.ToLower(p.Title + " " + p.Context + " " + p.Handle)
	matched := false
	for _, supp := range AllowedSupplements {
		if strings.Contains(identityString, supp) {
			matched = true
			break
		}
	}
	if !matched {
		return nil // Not a supplement we track — not a gap, just irrelevant
	}

	// --- Check if AnalyzeProduct already succeeds ---
	if AnalyzeProduct(vendorName, p) != nil {
		return nil // Product is fully analyzable, no audit needed
	}

	// --- The product IS interesting but the analyzer rejected it. Diagnose. ---
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
		price, _ := strconv.ParseFloat(v.Price, 64)
		if price <= 0 {
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

	// Use the same search strings as the analyzer to probe for data
	broadSearch := p.Title + " " + p.Context + " " + strings.ReplaceAll(p.Handle, "-", " ") + " " + p.BodyHTML
	for _, v := range p.Variants {
		broadSearch += " " + v.Title
	}
	cleanSearch := p.Title
	for _, v := range p.Variants {
		cleanSearch += " " + v.Title
	}

	// Probe: explicit grams
	gramMatch := reGrams.FindStringSubmatch(cleanSearch)
	if len(gramMatch) > 1 {
		g, _ := strconv.ParseFloat(gramMatch[1], 64)
		if g > 0 {
			result.GramsFound = true
			result.GramsValue = g
		}
	} else {
		// Fallback to broad search for grams
		gramMatchBroad := reGrams.FindStringSubmatch(broadSearch)
		if len(gramMatchBroad) > 1 {
			g, _ := strconv.ParseFloat(gramMatchBroad[1], 64)
			if g > 0 {
				result.GramsFound = true
				result.GramsValue = g
			}
		}
	}

	// Probe: kg
	kgMatch := reKg.FindStringSubmatch(cleanSearch)
	if len(kgMatch) > 1 {
		kg, _ := strconv.ParseFloat(kgMatch[1], 64)
		if kg > 0 {
			result.KgFound = true
			result.KgValue = kg
		}
	}

	// Probe: mg
	mgMatch := reMg.FindStringSubmatch(broadSearch)
	if len(mgMatch) > 1 {
		mg, _ := strconv.ParseFloat(mgMatch[1], 64)
		if mg > 0 {
			result.MgFound = true
			result.MgValue = mg
		}
	}

	// Probe: count
	variantSearch := ""
	for _, v := range p.Variants {
		variantSearch += " " + v.Title
	}
	countMatch := extractCount(variantSearch, cleanSearch, broadSearch)
	if len(countMatch) > 1 {
		c, _ := strconv.ParseFloat(countMatch[1], 64)
		if c > 0 {
			result.CountFound = true
			result.CountValue = c
		}
	}

	// --- Diagnose what's missing ---
	hasPowderMass := result.GramsFound || result.KgFound
	hasCapsuleMass := result.MgFound && result.CountFound

	if !hasPowderMass && !hasCapsuleMass {
		// Neither path can compute totalGrams
		if !result.MgFound {
			result.Missing = append(result.Missing, "mg per serving (forceMg)")
		}
		if !result.CountFound {
			result.Missing = append(result.Missing, "capsule/tablet count (forceCount)")
		}
		if !result.GramsFound && !result.KgFound {
			result.Missing = append(result.Missing, "total grams/kg (no powder weight found)")
		}
	} else {
		// We found partial data but totalGrams still came out zero or analysis
		// still failed for some other reason — flag it generically
		result.Missing = append(result.Missing, "data was partially found but totalGrams still computed to 0 (check overrides)")
	}

	return result
}

// FormatAuditReport produces a human-readable multi-line string from a slice
// of AuditResults, suitable for printing to stdout. It groups results by
// vendor and shows exactly what data is available and what needs an override.
func FormatAuditReport(results []AuditResult) string {
	if len(results) == 0 {
		return "✅ No gaps detected. All interesting products have enough data for analysis."
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("\n🔍 AUDIT: %d product(s) need manual overrides in data/vendor_rules.json\n", len(results)))
	b.WriteString(strings.Repeat("─", 80) + "\n")

	// Group by vendor
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

			// Suggest the override snippet
			b.WriteString("  │  Suggested override:\n")
			b.WriteString(fmt.Sprintf("  │    \"%s\": {\n", r.Handle))
			b.WriteString("  │      \"forceType\": \"Capsules\",\n")
			if !r.MgFound {
				b.WriteString("  │      \"forceMg\": ???,\n")
			} else {
				b.WriteString(fmt.Sprintf("  │      \"forceMg\": %.0f,\n", r.MgValue))
			}
			if !r.CountFound {
				b.WriteString("  │      \"forceCount\": ???\n")
			} else {
				b.WriteString(fmt.Sprintf("  │      \"forceCount\": %.0f\n", r.CountValue))
			}
			b.WriteString("  │    }\n")
			b.WriteString("  │\n")
		}
	}
	b.WriteString(strings.Repeat("─", 80) + "\n")
	return b.String()
}