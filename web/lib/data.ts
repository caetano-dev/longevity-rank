/**
 * Data loader — reads the pre-computed analysis report from the Go backend.
 *
 * The ONLY file this module touches is data/analysis_report.json, which is
 * the sole integration point between the Go scraper and the Next.js frontend.
 *
 * Snake_case JSON fields from the Go output are mapped to camelCase here.
 * Everything downstream of this module uses clean camelCase Analysis objects.
 *
 * This module is only used in Server Components / SSG (Static Site Generation).
 */

import fs from "fs";
import path from "path";

import type { Analysis } from "./types";

/** Raw shape of each entry in analysis_report.json (Go JSON tags are snake_case). */
interface RawReportEntry {
  vendor: string;
  name: string;
  handle: string;
  price: number;
  total_grams: number;
  cost_per_gram: number;
  effective_cost: number;
  type: string;
  image_url: string;
}

/** Absolute path to the /data directory at the repo root. */
const DATA_DIR = path.resolve(process.cwd(), "..", "data");

/**
 * Map a single raw JSON entry (snake_case) to the camelCase Analysis type
 * used by all frontend components.
 */
function mapEntry(raw: RawReportEntry): Analysis {
  return {
    vendor: raw.vendor,
    name: raw.name,
    handle: raw.handle,
    price: raw.price,
    totalGrams: raw.total_grams,
    costPerGram: raw.cost_per_gram,
    effectiveCost: raw.effective_cost,
    type: raw.type,
    imageURL: raw.image_url,
  };
}

/**
 * Load the pre-computed analysis report produced by the Go backend.
 * Returns an array of Analysis entries sorted by effectiveCost ascending
 * (the Go backend already sorts, but order is preserved).
 *
 * Returns an empty array if the file is missing or malformed.
 */
export function loadReport(): Analysis[] {
  const filePath = path.join(DATA_DIR, "analysis_report.json");
  try {
    const raw = fs.readFileSync(filePath, "utf-8");
    const parsed: unknown = JSON.parse(raw);
    if (!Array.isArray(parsed)) {
      return [];
    }
    return (parsed as RawReportEntry[]).map(mapEntry);
  } catch {
    // File missing or malformed — return empty so the build doesn't crash
    return [];
  }
}