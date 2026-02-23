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
	reGrams   = regexp.MustCompile(`(?i)(\d+)\s*(?:grams?|gms?|g)\b`)
	reKg      = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*kg\b`)
	rePack    = regexp.MustCompile(`(?i)(\d+)\s*(?:Pack|Bottles?)`)
	reServing = regexp.MustCompile(`(?i)(\d+)\s*(?:capsules|caps).*?per\s*serving`)

	// reLabelGrams and reLabelKg scan only variant.Title and product.Title (label text)
	// for Gross Grams extraction. Identical patterns to reGrams/reKg but kept separate
	// for clarity of intent.
	reLabelGrams = regexp.MustCompile(`(?i)(\d+)\s*(?:grams?|gms?|g)\b`)
	reLabelKg    = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*kg\b`)
)

// dirtyKeywords flags products whose regex-extracted mass is likely unreliable.
var dirtyKeywords = []string{
	"flavor", "island cooler", "coastal explosion", "watermelon", "berry", "punch",
	"orange", "lemon", "mango", "grape", "apple", "blend", "complex", "with", "+",
	"gumm", "chew", "bundle", "blue raspberry", "fruit punch", "sour watermelon",
	"pineapple mango", "mandarin orange", "shaq's berry blast", "frozen lemonade",
}

// Analyzer holds the configuration needed by the analysis and audit pipelines.
// There is no global mutable state — all dependencies are injected here.
type Analyzer struct {
	Rules       rules.Registry
	Supplements []string
}

// matchesSupplement reports whether the product's identity string contains at
// least one of the configured supplement keywords.
func (a *Analyzer) matchesSupplement(identity string) bool {
	return containsAny(identity, a.Supplements)
}

// vendorConfig returns the VendorConfig for the given vendor name, plus the
// product-level spec and whether an override exists for the given handle.
func (a *Analyzer) vendorConfig(vendorName, handle string) (cfg rules.VendorConfig, spec rules.ProductSpec, hasOverride bool) {
	if a.Rules == nil {
		return
	}
	cfg, exists := a.Rules[vendorName]
	if !exists {
		return
	}
	spec, hasOverride = cfg.Overrides[handle]
	return
}

// AnalyzeProduct evaluates every available variant of a product and returns an
// Analysis entry for each valid one. It implements a Hybrid Catalog/Regex Engine:
//
//   - If the product handle has an override with ForceActiveGrams > 0, the regex
//     mass-extraction pipeline is bypassed entirely.
//   - The pack multiplier regex (rePack) always runs regardless of overrides.
//   - When GlobalSubscriptionDiscount is configured, a synthetic "Subscribe & Save"
//     entry is emitted for each variant.
//
// Returns nil when the product has no variants, does not match any allowed
// supplement keyword, or yields no valid analyses.
func (a *Analyzer) AnalyzeProduct(vendorName string, p models.Product) []models.Analysis {
	if len(p.Variants) == 0 {
		return nil
	}

	identity := strings.ToLower(p.Title + " " + p.Context + " " + p.Handle)
	if !a.matchesSupplement(identity) {
		return nil
	}

	cfg, spec, hasOverride := a.vendorConfig(vendorName, p.Handle)

	var results []models.Analysis

	for _, v := range p.Variants {
		if !v.Available {
			continue
		}

		// Variant-level blocklist
		if len(cfg.VariantBlocklist) > 0 && containsAny(strings.ToLower(v.Title), cfg.VariantBlocklist) {
			continue
		}

		price, err := strconv.ParseFloat(v.Price, 64)
		if err != nil || price <= 0 {
			continue
		}

		// --- Search strings at different specificity levels ---
		variantSearch := v.Title
		cleanSearch := p.Title + " " + v.Title
		broadSearch := p.Title + " " + p.Context + " " + v.Title + " " + strings.ReplaceAll(p.Handle, "-", " ") + " " + p.BodyHTML

		// =================================================================
		// ACTIVE GRAMS EXTRACTION — Hybrid Engine
		// =================================================================
		capsuleMass, powderMass, usedOverride := a.extractMass(spec, hasOverride, v.Title, cleanSearch, broadSearch, variantSearch)

		baseMass := capsuleMass + powderMass

		// =================================================================
		// PACK MULTIPLIER — Always runs regardless of override source
		// =================================================================
		packMultiplier := 1.0
		if m, ok := extractFloatFrom(rePack, variantSearch, broadSearch); ok {
			packMultiplier = m
		}

		activeGrams := baseMass * packMultiplier
		if activeGrams <= 0 {
			continue
		}

		// =================================================================
		// GROSS GRAMS EXTRACTION — Label Weight
		// =================================================================
		isCapsuleProduct := capsuleMass > 0 && powderMass == 0
		grossGrams := a.extractGrossGrams(spec, hasOverride, v.Title, p.Title, isCapsuleProduct, packMultiplier)

		// =================================================================
		// PURE POWDER FALLBACK
		// =================================================================
		if !usedOverride && grossGrams > 0 && !isCapsuleProduct {
			triageTarget := strings.ToLower(p.Title + " " + v.Title + " " + p.Handle)
			if !containsAny(triageTarget, dirtyKeywords) {
				activeGrams = grossGrams
			}
		}

		// =================================================================
		// TYPE DETERMINATION — Hybrid Engine
		// =================================================================
		typeSearch := strings.ToLower(p.Title + " " + v.Title + " " + p.Handle + " " + p.Context)
		productType := classifyType(typeSearch, spec, hasOverride, usedOverride, packMultiplier, capsuleMass, powderMass)

		// --- Bioavailability multiplier ---
		multiplier, multiplierLabel := bioavailabilityMultiplier(typeSearch, productType)

		// --- Display name ---
		displayName := buildDisplayName(p.Title, v.Title, vendorName)

		// =================================================================
		// TRIAGE ENGINE — Dirty Data Detection
		// =================================================================
		needsReview, reviewReason := a.triageDirtyData(usedOverride, displayName, p.Handle, p.Title)

		// Pure powder gross fallback
		if productType == "Powder" && grossGrams == 0 && !needsReview {
			grossGrams = activeGrams
		}

		// --- One-time purchase entry ---
		results = append(results, buildAnalysis(
			vendorName, displayName, p.Handle, p.ImageURL, productType,
			price, activeGrams, grossGrams, multiplier, multiplierLabel,
			false, needsReview, reviewReason,
		))

		// --- Synthetic subscription entry ---
		if cfg.GlobalSubscriptionDiscount > 0 {
			subPrice := price * (1 - cfg.GlobalSubscriptionDiscount)
			results = append(results, buildAnalysis(
				vendorName, displayName+" (Subscribe & Save)", p.Handle, p.ImageURL, productType,
				subPrice, activeGrams, grossGrams, multiplier, multiplierLabel,
				true, needsReview, reviewReason,
			))
		}
	}

	if len(results) == 0 {
		return nil
	}
	return results
}

// extractMass implements the hybrid catalog/regex mass-extraction pipeline.
// Returns capsuleMass, powderMass, and whether an override was used.
func (a *Analyzer) extractMass(spec rules.ProductSpec, hasOverride bool, variantTitle, cleanSearch, broadSearch, variantSearch string) (capsuleMass, powderMass float64, usedOverride bool) {
	// VARIANT CATALOG PATH
	if hasOverride && spec.VariantOverrides != nil && spec.VariantOverrides[variantTitle] > 0 {
		return 0, spec.VariantOverrides[variantTitle], true
	}

	// PRODUCT CATALOG PATH
	if hasOverride && spec.ForceActiveGrams > 0 {
		return 0, spec.ForceActiveGrams, true
	}

	// REGEX PATH

	// Step 1: Explicit grams or kg in clean title+variant
	if g, ok := extractFloat(reGrams, cleanSearch); ok {
		return 0, g, false
	}
	if kg, ok := extractFloat(reKg, cleanSearch); ok {
		return 0, kg * 1000.0, false
	}

	// Step 2: mg × count (capsules/tablets)
	mg, mgOk := extractFloat(reMg, broadSearch)
	count, countOk := extractFloatFrom(reCount, variantSearch, cleanSearch, broadSearch)
	if mgOk && countOk {
		servingSize := 1.0
		if s, ok := extractFloat(reServing, broadSearch); ok {
			servingSize = s
		}
		capsuleMass = (mg / servingSize * count) / 1000.0
		return capsuleMass, 0, false
	}

	// Step 3: Fallback — grams in broad search
	if g, ok := extractFloat(reGrams, broadSearch); ok {
		return 0, g, false
	}

	return 0, 0, false
}

// extractGrossGrams extracts the physical label weight from variant/product titles.
func (a *Analyzer) extractGrossGrams(spec rules.ProductSpec, hasOverride bool, variantTitle, productTitle string, isCapsule bool, packMult float64) float64 {
	// Variant-level gross override
	if hasOverride && spec.VariantGrossOverrides != nil && spec.VariantGrossOverrides[variantTitle] > 0 {
		return spec.VariantGrossOverrides[variantTitle]
	}

	// Capsules don't have a meaningful gross weight
	if isCapsule {
		return 0
	}

	labelSearch := productTitle + " " + variantTitle
	if g, ok := extractFloat(reLabelGrams, labelSearch); ok {
		return g * packMult
	}
	if kg, ok := extractFloat(reLabelKg, labelSearch); ok {
		return kg * 1000.0 * packMult
	}
	return 0
}

// classifyType determines the product type string.
func classifyType(typeSearch string, spec rules.ProductSpec, hasOverride, usedOverride bool, packMult, capsuleMass, powderMass float64) string {
	if hasOverride && spec.ForceType != "" {
		return spec.ForceType
	}
	if packMult > 1 {
		return "Multi-Pack"
	}
	if !usedOverride && capsuleMass > 0 && powderMass > 0 {
		return "Hybrid Bundle"
	}
	if !usedOverride && powderMass > 0 && capsuleMass == 0 {
		return "Powder"
	}
	if strings.Contains(typeSearch, "gel") && !strings.Contains(typeSearch, "softgel") {
		return "Gel"
	}
	if strings.Contains(typeSearch, "tab") {
		return "Tablets"
	}
	if strings.Contains(typeSearch, "powder") {
		return "Powder"
	}
	return "Capsules"
}

// bioavailabilityMultiplier returns the multiplier and label for delivery bonuses.
func bioavailabilityMultiplier(typeSearch, productType string) (float64, string) {
	switch {
	case strings.Contains(typeSearch, "liposomal") || strings.Contains(typeSearch, "lipo"):
		return 1.5, "Lipo Bonus"
	case strings.Contains(typeSearch, "sublingual"):
		return 1.1, "Sublingual"
	case productType == "Gel":
		return 1.1, "Gel Bonus"
	case productType == "Tablets":
		return 1.1, "Tablet Bonus"
	default:
		return 1.0, ""
	}
}

// buildDisplayName constructs the user-facing product name, stripping the
// redundant vendor name prefix and appending the variant title when meaningful.
func buildDisplayName(productTitle, variantTitle, vendorName string) string {
	name := productTitle
	if variantTitle != "" && !strings.EqualFold(variantTitle, "Default Title") {
		name += " (" + variantTitle + ")"
	}

	// Strip redundant vendor name prefix (case-insensitive)
	if len(vendorName) > 0 && len(name) >= len(vendorName) &&
		strings.EqualFold(name[:len(vendorName)], vendorName) {
		if trimmed := strings.TrimSpace(name[len(vendorName):]); len(trimmed) > 0 {
			name = trimmed
		}
	}
	return name
}

// triageDirtyData checks whether regex-extracted mass is likely unreliable.
func (a *Analyzer) triageDirtyData(usedOverride bool, displayName, handle, title string) (bool, string) {
	if usedOverride {
		return false, ""
	}

	triageTarget := strings.ToLower(displayName + " " + handle + " " + title)
	for _, kw := range dirtyKeywords {
		if !strings.Contains(triageTarget, strings.ToLower(kw)) {
			continue
		}
		// "unflavored" contains substring "flavor" but is not a dirty signal
		if kw == "flavor" && strings.Contains(triageTarget, "unflavored") {
			if strings.Contains(triageTarget, "serv") {
				return true, "Detected 'unflavored' but uses 'servings' (needs manual math check)"
			}
			continue
		}
		return true, "Detected dirty keyword: " + kw
	}
	return false, ""
}

// buildAnalysis constructs a single Analysis entry with computed cost metrics.
func buildAnalysis(
	vendor, name, handle, imageURL, productType string,
	price, activeGrams, grossGrams, multiplier float64, multiplierLabel string,
	isSubscription, needsReview bool, reviewReason string,
) models.Analysis {
	costPerGram := price / activeGrams
	return models.Analysis{
		Vendor:          vendor,
		Name:            name,
		Handle:          handle,
		Price:           price,
		ActiveGrams:     activeGrams,
		GrossGrams:      grossGrams,
		CostPerGram:     costPerGram,
		EffectiveCost:   costPerGram / multiplier,
		Multiplier:      multiplier,
		MultiplierLabel: multiplierLabel,
		Type:            productType,
		ImageURL:        imageURL,
		IsSubscription:  isSubscription,
		NeedsReview:     needsReview,
		ReviewReason:    reviewReason,
	}
}