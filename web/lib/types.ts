// Mirrors Go struct: internal/models/types.go

export interface Variant {
  price: string;
  title: string;
  available: boolean;
}

export interface Product {
  id: string;
  title: string;
  context: string;
  handle: string;
  body_html: string;
  image_url: string;
  variants: Variant[];
}

export interface Analysis {
  vendor: string;
  name: string;
  price: number;
  totalGrams: number;
  costPerGram: number;
  effectiveCost: number;
  type: string;
  imageURL: string;
  handle: string;
}

// Mirrors Go struct: internal/rules/rules.go

export interface ProductSpec {
  forceType: string;
  forceMg: number;
  forceCount: number;
}

export interface VendorConfig {
  blocklist: string[];
  overrides: Record<string, ProductSpec>;
}

export type VendorRules = Record<string, VendorConfig>;

// Vendor registry entry (mirrors internal/config/vendors.go)

export interface VendorEntry {
  name: string;
  baseURL: string;
  scrapeURL: string;
  type: "shopify" | "magento" | "html-ldjson";
  cloudflare: boolean;
  dataFile: string;
}