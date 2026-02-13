/**
 * TypeScript port of internal/parser/analyzer.go
 *
 * Replicates the Go math engine: regex extraction of mg, count, grams, kg,
 * pack multiplier, serving size, type classification, bioavailability multiplier,
 * and effective cost calculation.
 *
 * Every regex and priority order matches the Go implementation exactly.
 */

import type { Product, Variant, Analysis, VendorRules } from "./types";

// --- Regexes (mirrors Go `var` block) ---

const reMg = /(\d+)\s*mg/i;
const reCount = /(\d+)\s*(?:capsules|caps|servings|tabs|tablets|ct)/i;
const reGrams = /(\d+)\s*(?:grams?|g)\b/i;
const reKg = /(\d+(?:\.\d+)?)\s*kg\b/i;
const rePack = /(\d+)\s*Pack/i;
const reServing = /(\d+)\s*(?:capsules|caps).*?per\s*serving/i;

// --- Allowed supplements (mirrors Go AllowedSupplements) ---

const DEFAULT_SUPPLEMENTS = [
  "nmn",
  "nad",
  "tmg",
  "trimethylglycine",
  "resveratrol",
  "creatine",
];

/**
 * extractCount replicates the Go helper of the same name.
 * It tries progressively broader search strings and returns the first match.
 */
function extractCount(
  variantSearch: string,
  cleanSearch: string,
  broadSearch: string
): RegExpMatchArray | null {
  // Priority 1: variant title alone
  let m = variantSearch.match(reCount);
  if (m) return m;

  // Priority 2: product title + variant title
  m = cleanSearch.match(reCount);
  if (m) return m;

  // Priority 3: everything
  m = broadSearch.match(reCount);
  if (m) return m;

  return null;
}

/**
 * applyRules replicates rules.ApplyRules from Go.
 * Returns false if the product should be rejected (blocklisted).
 * Mutates `product.context` in place when overrides match.
 */
export function applyRules(
  vendorName: string,
  product: Product,
  rules: VendorRules
): boolean {
  const config = rules[vendorName];
  if (!config) return true;

  // 1. Check blocklist
  const identity = (
    product.title +
    " " +
    product.handle +
    " " +
    product.context
  ).toLowerCase();

  for (const blocked of config.blocklist ?? []) {
    if (identity.includes(blocked.toLowerCase())) {
      return false;
    }
  }

  // 2. Apply overrides
  const spec = config.overrides?.[product.handle];
  if (spec) {
    let extraContext = "";
    if (spec.forceMg > 0) {
      extraContext += ` ${spec.forceMg}mg`;
    }
    if (spec.forceCount > 0) {
      if (spec.forceType === "Powder") {
        extraContext += ` ${spec.forceCount}g`;
      } else if (spec.forceType === "Tablets") {
        extraContext += ` ${spec.forceCount} tablets`;
      } else {
        extraContext += ` ${spec.forceCount} capsules`;
      }
    }
    product.context += extraContext;
  }

  return true;
}

/**
 * analyzeProduct is a line-for-line port of Go's AnalyzeProduct.
 *
 * Given a vendor name and product, it iterates over available variants,
 * extracts dosage information from progressively broader search strings,
 * computes cost-per-gram and effective cost (with bioavailability multiplier),
 * and returns the best (lowest $/g) analysis — or null if nothing matched.
 */
export function analyzeProduct(
  vendorName: string,
  product: Product,
  allowedSupplements: string[] = DEFAULT_SUPPLEMENTS
): Analysis | null {
  if (!product.variants || product.variants.length === 0) {
    return null;
  }

  // --- Supplement keyword filter ---
  const identityString = (
    product.title +
    " " +
    product.context +
    " " +
    product.handle
  ).toLowerCase();

  let matched = false;
  for (const supp of allowedSupplements) {
    if (identityString.includes(supp)) {
      matched = true;
      break;
    }
  }
  if (!matched) return null;

  let bestAnalysis: Analysis | null = null;
  let minCostPerGram = Number.MAX_VALUE;

  for (const v of product.variants) {
    if (!v.available) continue;

    const price = parseFloat(v.price);
    if (!price || price <= 0) continue;

    // --- Build search strings at different specificity levels ---
    const variantSearch = v.title;
    const cleanSearch = product.title + " " + v.title;
    const broadSearch =
      product.title +
      " " +
      product.context +
      " " +
      v.title +
      " " +
      product.handle.replace(/-/g, " ") +
      " " +
      product.body_html;

    let capsuleMass = 0;
    let powderMass = 0;
    let packMultiplier = 1;

    // --- Step 1: Check for explicit grams or kg in clean title+variant ---
    const gramMatch = cleanSearch.match(reGrams);
    const kgMatch = cleanSearch.match(reKg);

    if (gramMatch) {
      powderMass = parseFloat(gramMatch[1]);
    } else if (kgMatch) {
      powderMass = parseFloat(kgMatch[1]) * 1000;
    } else {
      // --- Step 2: Extract mg and capsule count ---
      const mgMatch = broadSearch.match(reMg);
      const countMatch = extractCount(variantSearch, cleanSearch, broadSearch);

      if (mgMatch && countMatch) {
        const mg = parseFloat(mgMatch[1]);
        const count = parseFloat(countMatch[1]);

        const servingMatch = broadSearch.match(reServing);
        let servingSize = 1;
        if (servingMatch) {
          const s = parseFloat(servingMatch[1]);
          if (s > 0) servingSize = s;
        }

        capsuleMass = (mg / servingSize * count) / 1000;
      }
    }

    // --- Step 3: Fallback — check broad search for grams ---
    if (powderMass === 0 && capsuleMass === 0) {
      const gramMatchBody = broadSearch.match(reGrams);
      if (gramMatchBody) {
        powderMass = parseFloat(gramMatchBody[1]);
      }
    }

    // --- Step 4: Pack multiplier ---
    let packMatch = variantSearch.match(rePack);
    if (!packMatch) {
      packMatch = broadSearch.match(rePack);
    }
    if (packMatch) {
      packMultiplier = parseFloat(packMatch[1]);
    }

    const totalGrams = (capsuleMass + powderMass) * packMultiplier;
    if (totalGrams <= 0) continue;

    const costPerGram = price / totalGrams;

    // --- Type classification (never uses BodyHTML to avoid HTML tag leakage) ---
    const typeSearch = (
      product.title +
      " " +
      v.title +
      " " +
      product.handle +
      " " +
      product.context
    ).toLowerCase();

    let productType = "Single";

    if (packMultiplier > 1) {
      productType = "Multi-Pack";
    } else if (capsuleMass > 0 && powderMass > 0) {
      productType = "Hybrid Bundle";
    } else if (powderMass > 0) {
      productType = "Powder";
    } else if (
      typeSearch.includes("gel") &&
      !typeSearch.includes("softgel")
    ) {
      productType = "Gel";
    } else if (typeSearch.includes("tab")) {
      productType = "Tablets";
    } else {
      productType = "Capsules";
    }

    // --- Bioavailability multiplier ---
    let multiplier = 1.0;

    if (
      typeSearch.includes("liposomal") ||
      typeSearch.includes("lipo")
    ) {
      multiplier = 1.5;
    } else if (
      typeSearch.includes("sublingual") ||
      productType === "Gel" ||
      productType === "Tablets"
    ) {
      multiplier = 1.1;
    }

    const effectiveCost = costPerGram / multiplier;

    if (costPerGram < minCostPerGram) {
      minCostPerGram = costPerGram;

      let displayName = product.title;
      if (
        v.title &&
        v.title.toLowerCase() !== "default title"
      ) {
        displayName = displayName + " (" + v.title + ")";
      }

      bestAnalysis = {
        vendor: vendorName,
        name: displayName,
        price,
        totalGrams,
        costPerGram,
        effectiveCost,
        type: productType,
        imageURL: product.image_url,
        handle: product.handle,
      };
    }
  }

  return bestAnalysis;
}

/**
 * Analyze all products for a single vendor, applying rules first.
 * Returns an array of Analysis results (one per qualifying product).
 */
export function analyzeVendor(
  vendorName: string,
  products: Product[],
  rules: VendorRules,
  allowedSupplements?: string[]
): Analysis[] {
  const results: Analysis[] = [];

  for (const product of products) {
    // Deep-clone so context mutation from applyRules doesn't pollute the source
    const p: Product = JSON.parse(JSON.stringify(product));

    if (!applyRules(vendorName, p, rules)) continue;

    const analysis = analyzeProduct(vendorName, p, allowedSupplements);
    if (analysis) {
      results.push(analysis);
    }
  }

  return results;
}

/**
 * Analyze all vendors at once. Returns a flat array sorted by effectiveCost ascending.
 */
export function analyzeAll(
  vendorProducts: { vendorName: string; products: Product[] }[],
  rules: VendorRules,
  allowedSupplements?: string[]
): Analysis[] {
  const all: Analysis[] = [];

  for (const { vendorName, products } of vendorProducts) {
    all.push(...analyzeVendor(vendorName, products, rules, allowedSupplements));
  }

  all.sort((a, b) => a.effectiveCost - b.effectiveCost);
  return all;
}