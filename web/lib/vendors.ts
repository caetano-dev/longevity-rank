/**
 * Vendor registry — maps vendor names to their base URLs for affiliate link construction.
 *
 * baseUrl is the storefront root (no trailing slash) used to construct affiliate links.
 * handleIsFullUrl: if true, product.handle is already a full URL (Magento / LD+JSON vendors).
 *
 * This module no longer references individual data files. The frontend reads
 * exclusively from data/analysis_report.json via lib/data.ts.
 */

export interface VendorInfo {
  name: string;
  baseUrl: string;
  handleIsFullUrl: boolean;
}

const vendors: VendorInfo[] = [
  {
    name: "ProHealth",
    baseUrl: "https://www.prohealth.com",
    handleIsFullUrl: false,
  },
  {
    name: "Renue By Science",
    baseUrl: "https://renuebyscience.com",
    handleIsFullUrl: false,
  },
  {
    name: "NMN Bio",
    baseUrl: "https://nmnbio.co.uk",
    handleIsFullUrl: false,
  },
  {
    name: "Jinfiniti",
    baseUrl: "https://www.jinfiniti.com",
    handleIsFullUrl: true,
  },
  {
    name: "Do Not Age",
    baseUrl: "https://donotage.org",
    handleIsFullUrl: true,
  },
  {
    name: "Nutricost",
    baseUrl: "https://nutricost.com",
    handleIsFullUrl: false,
  },
  {
    name: "Wonderfeel",
    baseUrl: "https://www.wonderfeel.com",
    handleIsFullUrl: true,
  },
];

export default vendors;

/**
 * Build a product URL from vendor info + product handle.
 * Shopify vendors: {baseUrl}/products/{handle}
 * Full-URL vendors: handle is used as-is.
 */
export function buildProductUrl(vendor: VendorInfo, handle: string): string {
  if (vendor.handleIsFullUrl) {
    return handle;
  }
  return `${vendor.baseUrl}/products/${handle}`;
}

/**
 * Append an affiliate query parameter to a product URL.
 * Returns the bare URL unchanged when affiliateId is empty/undefined.
 */
export function buildAffiliateUrl(
  vendor: VendorInfo,
  handle: string,
  affiliateId?: string
): string {
  const base = buildProductUrl(vendor, handle);
  if (!affiliateId) return base;

  const separator = base.includes("?") ? "&" : "?";
  return `${base}${separator}ref=${affiliateId}`;
}