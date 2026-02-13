/**
 * Analysis row — the component-facing type used by all frontend code.
 *
 * Field names are camelCase for ergonomic JSX access. The snake_case → camelCase
 * mapping from data/analysis_report.json happens once in lib/data.ts.
 *
 * This is the ONLY data type the frontend consumes. All parsing, regex,
 * bioavailability math, and type classification live in the Go backend.
 * The frontend is a dumb renderer.
 */
export interface Analysis {
  vendor: string;
  name: string;
  handle: string;
  price: number;
  totalGrams: number;
  costPerGram: number;
  effectiveCost: number;
  multiplier: number;
  multiplierLabel: string;
  type: string;
  imageURL: string;
}