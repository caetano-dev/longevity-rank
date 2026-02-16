package parser

import (
	"regexp"
	"strconv"
	"strings"

	"longevity-ranker/internal/models"
	"longevity-ranker/internal/rules"
)

var (
	reMg      = regexp.MustCompile(`(?i)(\d+)\s*mg`)
	reCount   = regexp.MustCompile(`(?i)(\d+)\s*(?:capsules|caps|servings|tabs|tablets|ct)`)
	reGrams   = regexp.MustCompile("(?i)(\\d+)\\s*(?:grams?|gms?|g)\\b")
	reKg      = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*kg\b`)
	rePack    = regexp.MustCompile("(?i)(\\d+)\\s*(?:Pack|Bottles?)")
	reServing = regexp.MustCompile(`(?i)(\d+)\s*(?:capsules|caps).*?per\s*serving`)
)

// dirtyKeywords flags products whose regex-extracted mass is likely unreliable.
// Flavored powders, blends, gummies, and multi-ingredient combos all have
// advertised weights that include non-active fillers. If no manual override
// exists, the product is marked NeedsReview so an operator can add one.
var dirtyKeywords = []string{
	"flavor", "watermelon", "berry", "punch", "orange", "lemon", "mango",
	"grape", "apple", "blend", "complex", "with", "+", "gumm", "chew", "bundle",
}

// AllowedSupplements controls which supplement keywords the analyzer will accept.
// Products must contain at least one of these in their identity string to be analyzed.
var AllowedSupplements = []string{"nmn", "nad", "tmg", "trimethylglycine", "resveratrol", "creatine"}

// AnalyzeProduct evaluates every available variant of a product and returns an
// Analysis entry for each valid one. It implements a Hybrid Catalog/Regex Engine:
//
//   - If the product handle has an override in vendor_rules.json with ForceTotalGrams > 0,
//     the regex mass-extraction pipeline is bypassed entirely and the override value is
//     used as the base mass.
//   - If the override has a ForceType, it is used directly; otherwise, the existing
//     string-matching logic determines the product type.
//   - The pack multiplier regex (rePack) always runs regardless of overrides.
//
// When the vendor has a GlobalSubscriptionDiscount configured in vendor_rules.json,
// a synthetic "Subscribe & Save" entry is also emitted for each variant.
// Returns nil when the product has no variants, does not match any allowed supplement
// keyword, or yields no valid analyses.
func AnalyzeProduct(vendorName string, p models.Product) []models.Analysis {
	if len(p.Variants) == 0 {
		return nil
	}

	identityString := strings.ToLower(p.Title + " " + p.Context + " " + p.Handle)
	matched := false
	for _, supp := range AllowedSupplements {
		if strings.Contains(identityString, supp) {
			matched = true
			break
		}
	}
	if !matched {
		return nil
	}

	// --- Look up vendor config for overrides and subscription discount ---
	var subscriptionDiscount float64
	var spec rules.ProductSpec
	var hasOverride bool
	var variantBlocklist []string

	if rules.Registry != nil {
		if config, exists := rules.Registry[vendorName]; exists {
			subscriptionDiscount = config.GlobalSubscriptionDiscount
			variantBlocklist = config.VariantBlocklist
			spec, hasOverride = config.Overrides[p.Handle]
		}
	}

	var results []models.Analysis

	for _, v := range p.Variants {
		if !v.Available {
			continue
		}

		// --- Variant-level blocklist (skip ghost variants) ---
		if len(variantBlocklist) > 0 {
			variantLower := strings.ToLower(v.Title)
			blocked := false
			for _, b := range variantBlocklist {
				if strings.Contains(variantLower, strings.ToLower(b)) {
					blocked = true
					break
				}
			}
			if blocked {
				continue
			}
		}

		price, _ := strconv.ParseFloat(v.Price, 64)
		if price <= 0 {
			continue
		}

		// --- Build search strings at different specificity levels ---
		// Level 1: Just variant title (most specific, e.g. "366 Capsules" or "60 Capsules - 3 Pack")
		variantSearch := v.Title

		// Level 2: Product title + variant title (e.g. "Pure NMN Supplement 366 Capsules")
		cleanSearch := p.Title + " " + v.Title

		// Level 3: Everything including context, handle, and body_html (broadest, most noise)
		broadSearch := p.Title + " " + p.Context + " " + v.Title + " " + strings.ReplaceAll(p.Handle, "-", " ") + " " + p.BodyHTML

		// =================================================================
		// MASS EXTRACTION PHASE — Hybrid Engine
		// =================================================================
		// capsuleMass and powderMass are hoisted here so type classification
		// can reference them later (e.g., to distinguish Powder vs Capsules).
		capsuleMass := 0.0
		powderMass := 0.0
		usedOverrideForMass := false

		if hasOverride && spec.VariantOverrides != nil && spec.VariantOverrides[v.Title] > 0 {
			// VARIANT CATALOG PATH: Per-variant override takes highest priority.
			// Bypasses both the product-level override and the regex pipeline.
			powderMass = spec.VariantOverrides[v.Title]
			usedOverrideForMass = true
		} else if hasOverride && spec.ForceTotalGrams > 0 {
			// PRODUCT CATALOG PATH: Override provides the immutable total grams.
			// Skip ALL regex mass extraction (reGrams, reKg, reMg, reCount, reServing).
			powderMass = spec.ForceTotalGrams
			usedOverrideForMass = true
		} else {
			// REGEX PATH: Standard extraction pipeline for ~80% of products.

			// Step 1: Check for explicit grams or kg in the clean title+variant
			gramMatch := reGrams.FindStringSubmatch(cleanSearch)
			kgMatch := reKg.FindStringSubmatch(cleanSearch)

			if len(gramMatch) > 1 {
				grams, _ := strconv.ParseFloat(gramMatch[1], 64)
				powderMass = grams
			} else if len(kgMatch) > 1 {
				kg, _ := strconv.ParseFloat(kgMatch[1], 64)
				powderMass = kg * 1000.0
			} else {
				// Step 2: Extract mg and capsule count
				mgMatch := reMg.FindStringSubmatch(broadSearch)
				countMatch := extractCount(variantSearch, cleanSearch, broadSearch)

				if len(mgMatch) > 1 && len(countMatch) > 1 {
					mg, _ := strconv.ParseFloat(mgMatch[1], 64)
					count, _ := strconv.ParseFloat(countMatch[1], 64)

					servingMatch := reServing.FindStringSubmatch(broadSearch)
					servingSize := 1.0
					if len(servingMatch) > 1 {
						s, _ := strconv.ParseFloat(servingMatch[1], 64)
						if s > 0 {
							servingSize = s
						}
					}
					capsuleMass = (mg / servingSize * count) / 1000.0
				}
			}

			// Step 3: Fallback — check broad search for grams if nothing found
			if powderMass == 0 && capsuleMass == 0 {
				gramMatchBody := reGrams.FindStringSubmatch(broadSearch)
				if len(gramMatchBody) > 1 {
					grams, _ := strconv.ParseFloat(gramMatchBody[1], 64)
					powderMass = grams
				}
			}
		}

		baseMass := capsuleMass + powderMass

		// =================================================================
		// PACK MULTIPLIER — Always runs regardless of override source
		// =================================================================
		packMultiplier := 1.0
		packMatch := rePack.FindStringSubmatch(variantSearch)
		if len(packMatch) == 0 {
			packMatch = rePack.FindStringSubmatch(broadSearch)
		}
		if len(packMatch) > 1 {
			mult, _ := strconv.ParseFloat(packMatch[1], 64)
			packMultiplier = mult
		}

		totalGrams := baseMass * packMultiplier
		if totalGrams <= 0 {
			continue
		}

		// =================================================================
		// TYPE DETERMINATION — Hybrid Engine
		// =================================================================
		typeSearch := strings.ToLower(p.Title + " " + v.Title + " " + p.Handle + " " + p.Context)
		productType := "Capsules" // sensible default

		if hasOverride && spec.ForceType != "" {
			// CATALOG PATH: Override dictates the type.
			productType = spec.ForceType
		} else if packMultiplier > 1 {
			productType = "Multi-Pack"
		} else if !usedOverrideForMass && capsuleMass > 0 && powderMass > 0 {
			productType = "Hybrid Bundle"
		} else if !usedOverrideForMass && powderMass > 0 && capsuleMass == 0 {
			productType = "Powder"
		} else if strings.Contains(typeSearch, "gel") && !strings.Contains(typeSearch, "softgel") {
			productType = "Gel"
		} else if strings.Contains(typeSearch, "tab") {
			productType = "Tablets"
		} else if strings.Contains(typeSearch, "powder") {
			productType = "Powder"
		} else {
			productType = "Capsules"
		}

		// --- Bioavailability multiplier ---
		multiplier := 1.0
		multiplierLabel := ""

		if strings.Contains(typeSearch, "liposomal") || strings.Contains(typeSearch, "lipo") {
			multiplier = 1.5
			multiplierLabel = "Lipo Bonus"
		} else if strings.Contains(typeSearch, "sublingual") {
			multiplier = 1.1
			multiplierLabel = "Sublingual"
		} else if productType == "Gel" {
			multiplier = 1.1
			multiplierLabel = "Gel Bonus"
		} else if productType == "Tablets" {
			multiplier = 1.1
			multiplierLabel = "Tablet Bonus"
		}

		// --- Build display name ---
		displayName := p.Title
		if v.Title != "" && !strings.EqualFold(v.Title, "Default Title") {
			displayName = displayName + " (" + v.Title + ")"
		}

		// Strip redundant vendor name prefix from display name (case-insensitive)
		trimmed := displayName
		if len(vendorName) > 0 && len(trimmed) >= len(vendorName) &&
			strings.EqualFold(trimmed[:len(vendorName)], vendorName) {
			trimmed = strings.TrimSpace(trimmed[len(vendorName):])
		}
		if len(trimmed) > 0 {
			displayName = trimmed
		}

		// =================================================================
		// TRIAGE ENGINE — Dirty Data Detection
		// =================================================================
		// If no override provided the mass, scan for dirty keywords that
		// indicate the regex-extracted weight is likely unreliable (flavored
		// powders, blends, gummies, etc.).
		needsReview := false
		reviewReason := ""

		if !usedOverrideForMass {
			triageTarget := strings.ToLower(displayName + " " + p.Handle + " " + p.Title)
			for _, kw := range dirtyKeywords {
				if strings.Contains(triageTarget, strings.ToLower(kw)) {
					needsReview = true
					reviewReason = "Detected dirty keyword: " + kw
					break
				}
			}
		}

		// --- One-time purchase entry ---
		costPerGram := price / totalGrams
		effectiveCost := costPerGram / multiplier

		results = append(results, models.Analysis{
			Vendor:          vendorName,
			Name:            displayName,
			Handle:          p.Handle,
			Price:           price,
			TotalGrams:      totalGrams,
			CostPerGram:     costPerGram,
			EffectiveCost:   effectiveCost,
			Multiplier:      multiplier,
			MultiplierLabel: multiplierLabel,
			Type:            productType,
			ImageURL:        p.ImageURL,
			IsSubscription:  false,
			NeedsReview:     needsReview,
			ReviewReason:    reviewReason,
		})

		// --- Synthetic subscription entry ---
		if subscriptionDiscount > 0 {
			subPrice := price * (1 - subscriptionDiscount)
			subCostPerGram := subPrice / totalGrams
			subEffectiveCost := subCostPerGram / multiplier

			results = append(results, models.Analysis{
				Vendor:          vendorName,
				Name:            displayName + " (Subscribe & Save)",
				Handle:          p.Handle,
				Price:           subPrice,
				TotalGrams:      totalGrams,
				CostPerGram:     subCostPerGram,
				EffectiveCost:   subEffectiveCost,
				Multiplier:      multiplier,
				MultiplierLabel: multiplierLabel,
				Type:            productType,
				ImageURL:        p.ImageURL,
				IsSubscription:  true,
				NeedsReview:     needsReview,
				ReviewReason:    reviewReason,
			})
		}
	}

	if len(results) == 0 {
		return nil
	}

	return results
}

// extractCount tries to find the capsule/tablet count from progressively broader
// search strings. The variant title is checked first because it is the most
// specific (e.g. "60 Capsules - 3 Pack"), avoiding contamination from ambiguous
// context strings like "60/366 Capsules".
func extractCount(variantSearch, cleanSearch, broadSearch string) []string {
	// Priority 1: variant title alone (e.g. "366 Capsules", "60 Capsules - 3 Pack")
	if m := reCount.FindStringSubmatch(variantSearch); len(m) > 1 {
		return m
	}
	// Priority 2: product title + variant title
	if m := reCount.FindStringSubmatch(cleanSearch); len(m) > 1 {
		return m
	}
	// Priority 3: full search string (title + context + handle + body)
	if m := reCount.FindStringSubmatch(broadSearch); len(m) > 1 {
		return m
	}
	return nil
}