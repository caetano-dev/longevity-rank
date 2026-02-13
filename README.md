# Longevity Ranker

A ruthless, high-performance aggregator that answers one question: **"Who has the cheapest authentic NMN (and other longevity supplements) today?"**

Architecture: **Git-Scraper**. Go scrapes vendor sites, runs the math engine, and writes a pre-computed `data/analysis_report.json`. Next.js reads that single file at build time and generates a static HTML page. $0/month infrastructure cost.

## Features

- **Multi-vendor price comparison** across Shopify, Magento, and LD+JSON storefronts.
- **Bioavailability-adjusted pricing** (True Cost) — liposomal, sublingual, and gel formulations receive a multiplier that lowers their effective $/gram. The multiplier value and label are exported in the JSON and displayed in the frontend's True Cost column as muted subtext (e.g., `1.5x Lipo Bonus`).
- **Clean product names** — the analyzer strips redundant vendor name prefixes from product titles (case-insensitive). E.g., vendor `"Nutricost"` + title `"Nutricost Creatine Monohydrate"` → `"Creatine Monohydrate"`.
- **Multi-supplement tracking** — NMN, NAD+, TMG, Resveratrol, and Creatine out of the box. Configurable via `--supplements` flag.
- **Cloudflare-safe** — vendors behind Cloudflare (Jinfiniti, Wonderfeel) are flagged with `Cloudflare: true` in the vendor config. The scraper skips them on `--refresh` and uses manually-maintained JSON instead.
- **Override system** — missing dosage data (capsule count, mg per cap) is fixed via hardcoded rules in `data/vendor_rules.json`. No OCR. No image parsing.
- **Pagination safety** — Shopify scraper uses proper URL construction, product deduplication, and a hard page limit (50) to prevent infinite loops.
- **Daily CI/CD** — GitHub Actions workflow scrapes daily, commits changed JSON, and triggers a Vercel build.

## Setup

### Backend (Go)

```
go mod tidy
```

### Frontend (Next.js)

```
cd web
npm install
```

## Usage

### Scrape data and rank products

```
go run cmd/main.go -refresh
```

### Rank products with local data (instant, no network)

```
go run cmd/main.go
```

### Audit products missing data (detect override gaps)

```
go run cmd/main.go -audit
go run cmd/main.go -refresh -audit
```

Scans all products that pass the supplement keyword filter and vendor blocklist, then reports any that lack enough data (mg, count, grams) for the analyzer to compute `totalGrams`. For each gap, prints the product handle, what data was extracted, what is missing, and a suggested `vendor_rules.json` override snippet. Use this after scraping to discover new products that need manual overrides.

### Filter by supplement type

```
go run cmd/main.go --supplements=nmn,nad
go run cmd/main.go --supplements=creatine
go run cmd/main.go --supplements=tmg,resveratrol
```

Default value: `nmn,nad,tmg,trimethylglycine,resveratrol,creatine`

### Build the frontend (static export)

```
cd web
npm run build
```

Output lands in `web/out/`. This is a fully static site — no server required.

### Run the frontend locally (dev mode)

```
cd web
npm run dev
```

Opens at `http://localhost:3000`. Reads `data/analysis_report.json` from the repo root at build time.

> **Prerequisite:** Run `go run cmd/main.go` at least once to generate `data/analysis_report.json` before building the frontend.

## Project Structure

```
cmd/main.go                  CLI entry point. Flags: --refresh, --supplements.
internal/
  config/vendors.go          Vendor registry (name, URL, scraper type, cloudflare flag).
  models/types.go            Core structs: Vendor, Product, Variant, Analysis (with JSON tags, including Multiplier and MultiplierLabel).
  parser/analyzer.go         Math engine. Extracts mg, count, grams from text. Calculates $/gram and True Cost. Strips vendor name from product title. Populates multiplier and multiplier label.
  parser/audit.go            Gap detector. Finds products that pass filters but lack data for analysis. Prints override suggestions.
  rules/rules.go             Loads vendor_rules.json. Applies blocklists and manual overrides.
  scraper/router.go          Routes vendors to the correct scraper engine.
  scraper/shopify.go         Shopify products.json scraper with pagination safety.
  scraper/magento.go         Magento swatch-renderer JSON + bulk pricing scraper.
  scraper/ld+json.go         Schema.org LD+JSON @graph scraper.
  storage/json_store.go      Reads/writes data/*.json files. SaveReport() writes analysis_report.json.
data/
  analysis_report.json       ★ THE INTEGRATION POINT. Pre-computed Analysis array. Frontend reads ONLY this.
  vendor_rules.json          Blocklists and manual dosage overrides per vendor.
  *.json                     Scraped raw product data (one file per vendor). NOT read by the frontend.
web/
  app/layout.tsx             Root layout. Dark theme, font loading, metadata.
  app/page.tsx               Main SSG page. Reads analysis_report.json, renders table. Zero parsing logic.
  app/globals.css            Tailwind v4 styles, rank badges, type badges, custom scrollbar.
  components/
    ProductTable.tsx          Desktop table + mobile card layout. Sorting, filtering, affiliate links.
    SupplementFilter.tsx      Pill-style filter tabs (All, NMN, NAD+, TMG, Resveratrol, Creatine).
    TypeBadge.tsx             Colored badge for product type (Capsules, Powder, Tablets, Gel, etc.).
    RankBadge.tsx             Gold/silver/bronze for top 3, plain number for the rest.
  lib/
    data.ts                  Reads data/analysis_report.json. Maps snake_case → camelCase. Single file.
    types.ts                 Analysis interface (camelCase). The only data type the frontend uses.
    vendors.ts               Vendor registry with base URLs. Affiliate link constructor.
  next.config.ts             Static export, remote image patterns for vendor CDNs.
  package.json               Next.js 15, React 19, Tailwind CSS v4.
  tsconfig.json              Path aliases (@/*).
.github/workflows/scrape.yml Daily cron job: scrape → diff → commit → deploy.
```

## Vendor Rules (`data/vendor_rules.json`)

Each vendor can have:

- **`blocklist`**: Product title substrings to reject (e.g. `"Bundle"`, `"Subscription"`).
- **`overrides`**: Keyed by product handle. Forces `forceType`, `forceMg`, and `forceCount` when the scraper cannot extract dosage from HTML.

Example:

```json
{
  "Renue By Science": {
    "blocklist": ["Cream", "Serum", "Pet"],
    "overrides": {
      "lipo-nmn-powdered-liposomal-nmn2": {
        "forceType": "Capsules",
        "forceMg": 250,
        "forceCount": 90
      }
    }
  }
}
```

## CI/CD

The GitHub Actions workflow (`.github/workflows/scrape.yml`) runs daily at 08:00 UTC:

1. Checks out the repo.
2. Runs `go run cmd/main.go -refresh`.
3. Diffs `data/*.json`. If unchanged, stops.
4. Commits changes as `"Auto-update product data [skip ci]"`.
5. Hits the Vercel deploy hook (requires `VERCEL_DEPLOY_HOOK` secret).

Manual trigger: use the **Run workflow** button on the Actions tab.

## Cloudflare-Protected Vendors

Jinfiniti and Wonderfeel are behind Cloudflare. Their `Cloudflare: true` flag causes the scraper to skip live fetching and load from `data/<vendor>.json` instead. To update their data:

1. Manually visit the vendor site.
2. Extract product info into the matching JSON file in `data/`.
3. Commit and push.

## Affiliate Link Generation

Product `Handle` (URL slug or full URL) and the vendor's base URL are available in the data. The frontend constructs affiliate links dynamically via `web/lib/vendors.ts`:

- **Shopify vendors** (ProHealth, Renue By Science, NMN Bio, Nutricost): `{baseUrl}/products/{handle}?ref={AFFILIATE_ID}`
- **Full-URL vendors** (Do Not Age, Jinfiniti, Wonderfeel): handle is the complete product URL, appended with `?ref={AFFILIATE_ID}`

`AFFILIATE_ID` is read from `process.env.AFFILIATE_ID` at build time. Set it as a Vercel environment variable or in a `.env.local` file. When empty, links point to the bare product URL with no query parameter.

## Data Pipeline

```
Go Scraper → data/*.json (raw) → analyzer.go → data/analysis_report.json → Next.js (dumb renderer)
```

The Go backend is the single source of truth for all parsing, regex extraction, bioavailability math, multiplier assignment, vendor-name stripping, and type classification. The frontend contains zero duplicated logic. It reads `data/analysis_report.json` and renders it.

## Frontend

**Stack:** Next.js 15 (App Router), React 19, Tailwind CSS v4, static export (`output: "export"`).

**How it works:**

1. `lib/data.ts` reads `data/analysis_report.json` using `fs.readFileSync` during the build step. It maps the Go backend's snake_case JSON fields to camelCase TypeScript via a `mapEntry()` function.
2. `app/page.tsx` calls `loadReport()`, attaches `VendorInfo` metadata (for affiliate links), and passes the result to `ProductTable`. No parsing. No math.
3. `ProductTable` is a client component that provides supplement filtering (pill tabs) and column sorting. Desktop shows a data table; mobile (<768px) shows a card layout. The True Cost column header has a hover tooltip `(i)` explaining: "Base Price ÷ Bioavailability Multiplier". When a product's `multiplier > 1`, a muted subtext below the True Cost value shows the multiplier and its label (e.g., `(1.5x Lipo Bonus)`).
4. The build produces a fully static `out/` directory. No server, no client-side API calls.

**Allowed frontend math:** Only user-driven state calculations (e.g., a future "Monthly Cost" column based on dosage input). All product-level computation is pre-computed by Go.

**Compliance banners** are rendered in the page footer: FDA disclaimer, EU Novel Food notice, and affiliate disclosure.