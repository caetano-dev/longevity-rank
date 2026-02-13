/**
 * Vendor registry — maps vendor names to their base URLs and data file names.
 * This mirrors internal/config/vendors.go on the Go side.
 *
 * baseUrl is the storefront root (no trailing slash) used to construct affiliate links.
 * dataFile is the filename inside /data (without path prefix).
 * handleIsFullUrl: if true, product.handle is already a full URL (Magento / LD+JSON vendors).
 */

export interface VendorInfo {
  name: string;
  baseUrl: string;
  dataFile: string;
  handleIsFullUrl: boolean;
}

const vendors: VendorInfo[] = [
  {
    name: "ProHealth",
    baseUrl: "https://www.prohealth.com",
    dataFile: "prohealth.json",
    handleIsFullUrl: false,
  },
  {
    name: "Renue By Science",
    baseUrl: "https://renuebyscience.com",
    dataFile: "renue_by_science.json",
    handleIsFullUrl: false,
  },
  {
    name: "NMN Bio",
    baseUrl: "https://nmnbio.co.uk",
    dataFile: "nmn_bio.json",
    handleIsFullUrl: false,
  },
  {
    name: "Jinfiniti",
    baseUrl: "https://www.jinfiniti.com",
    dataFile: "jinfiniti.json",
    handleIsFullUrl: true,
  },
  {
    name: "Do Not Age",
    baseUrl: "https://donotage.org",
    dataFile: "do_not_age.json",
    handleIsFullUrl: true,
  },
  {
    name: "Nutricost",
    baseUrl: "https://nutricost.com",
    dataFile: "nutricost.json",
    handleIsFullUrl: false,
  },
  {
    name: "Wonderfeel",
    baseUrl: "https://www.wonderfeel.com",
    dataFile: "wonderfeel.json",
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