/**
 * Data loader — reads all vendor JSON files and vendor_rules.json from the
 * filesystem at build time. This module is only used in Server Components
 * and during SSG (Static Site Generation).
 *
 * It reads from the repo-root /data directory (one level above /web).
 */

import fs from "fs";
import path from "path";

import type { Product, VendorRules } from "./types";
import vendors, { type VendorInfo } from "./vendors";

/** Absolute path to the /data directory at the repo root. */
const DATA_DIR = path.resolve(process.cwd(), "..", "data");

/**
 * Load and parse a JSON file from the /data directory.
 * Returns null if the file doesn't exist or contains invalid JSON.
 */
function loadJsonFile<T>(filename: string): T | null {
  const filePath = path.join(DATA_DIR, filename);
  try {
    const raw = fs.readFileSync(filePath, "utf-8");
    const parsed = JSON.parse(raw);
    // Some vendor files may contain `null` (e.g. jinfiniti.json when Cloudflare blocks scraping)
    if (parsed === null || parsed === undefined) {
      return null;
    }
    return parsed as T;
  } catch {
    // File missing or malformed — silently skip
    return null;
  }
}

/**
 * Load vendor_rules.json from /data.
 * Returns an empty object if the file is missing.
 */
export function loadVendorRules(): VendorRules {
  const rules = loadJsonFile<VendorRules>("vendor_rules.json");
  return rules ?? {};
}

/**
 * Load all product data for every vendor in the registry.
 * Returns an array of { vendorName, vendorInfo, products } tuples.
 * Vendors whose data file is missing or null are omitted.
 */
export function loadAllVendorProducts(): {
  vendorName: string;
  vendorInfo: VendorInfo;
  products: Product[];
}[] {
  const results: {
    vendorName: string;
    vendorInfo: VendorInfo;
    products: Product[];
  }[] = [];

  for (const vendor of vendors) {
    const products = loadJsonFile<Product[]>(vendor.dataFile);
    if (products && Array.isArray(products) && products.length > 0) {
      results.push({
        vendorName: vendor.name,
        vendorInfo: vendor,
        products,
      });
    }
  }

  return results;
}

/**
 * Convenience: load everything in one call — rules + all vendor products.
 */
export function loadAll(): {
  rules: VendorRules;
  vendorProducts: {
    vendorName: string;
    vendorInfo: VendorInfo;
    products: Product[];
  }[];
} {
  return {
    rules: loadVendorRules(),
    vendorProducts: loadAllVendorProducts(),
  };
}