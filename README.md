# Longevity Ranker

A ruthless, high-performance aggregator that answers one question: **"Who has the cheapest authentic NMN (and other longevity supplements) today?"**

Architecture: **Git-Scraper**. Go scrapes vendor sites, runs the math engine, and writes a pre-computed `data/analysis_report.json`. Next.js reads that single file at build time and generates a static HTML page. $0/month infrastructure cost.

## Features

- **Multi-vendor price comparison** across Shopify, Magento, and LD+JSON storefronts.
- **Bioavailability-adjusted pricing** (True Cost) — liposomal, sublingual, and gel formulations receive a multiplier that lowers their effective $/gram. The multiplier value and label are exported in the JSON and displayed in the frontend's True Cost column as muted subtext (e.g., `1.5x Lipo Bonus`).
- **Synthetic Subscription Pricing** — vendors whose Shopify APIs hide subscription prices (e.g., Renue By Science) are handled via a `globalSubscriptionDiscount` field in `data/vendor_rules.json`. The analyzer emits BOTH a one-time purchase entry and a synthetic "Subscribe & Save" entry (with `is_subscription: true`) for every valid variant. The frontend receives both rows and can toggle between purchase types.
- **Clean product names** — the analyzer strips redundant vendor name prefixes from product titles (case-insensitive). E.g., vendor `"Nutricost"` + title `"Nutricost Creatine Monohydrate"` → `"Creatine Monohydrate"`.
- **Multi-supplement tracking** — NMN, NAD+, TMG, Resveratrol, and Creatine out of the box. Configurable via `--supplements` flag.
- **Cloudflare-safe** — vendors behind Cloudflare (Jinfiniti, Wonderfeel) are flagged with `Cloudflare: true` in the vendor config. The scraper skips them on `--refresh` and uses manually-maintained JSON instead.
- **Hybrid Catalog/Regex Engine** — the analyzer uses a two-path architecture with active/gross mass disambiguation. ~80% of standard products are handled automatically by the regex extraction pipeline. The remaining ~20% of complex products (multi-ingredient, non-standard weights) are handled by immutable overrides in `data/vendor_rules.json` that bypass regex entirely. Overrides specify `forceActiveGrams` (the pre-computed total active ingredient mass) and optionally `forceType` and `forceServingMg`. `activeGrams` is the denominator for all cost calculations. `grossGrams` (the physical label weight) is resolved via a two-tier chain: `variantGrossOverrides` (manual per-variant override for titles lacking gram/kg patterns) > regex extraction from product/variant titles. No OCR. No image parsing. The same file supports `globalSubscriptionDiscount` for synthetic subscription price generation.
- **Triage Engine** — products whose mass was resolved by regex (no override) are scanned against a hardcoded `dirtyKeywords` list (flavors, blends, gummies, combos). A false-positive guard skips the `"flavor"` keyword when the target string contains `"unflavored"` — only that trigger is suppressed; the loop continues checking remaining keywords so that e.g. `"unflavored blend"` is still correctly flagged by `"blend"`. **Servings sub-exception:** before skipping the `"flavor"` match for an unflavored product, the engine checks if the target string also contains `"serv"`. If it does, the product is flagged with `review_reason: "Detected 'unflavored' but uses 'servings' (needs manual math check)"` — because servings-based sizing forces the regex to guess scoop size, making the computed mass mathematically unsafe. Only unflavored products with explicit gram/kg weights (e.g., `"Unflavored / 500 GMS"`) pass cleanly. Matches are flagged with `needs_review: true` and `review_reason` in the analysis output, and collected into `data/needs_review.json` for operator review. The triage is intentionally aggressive — it flags for human review, not rejection.
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

Scans all products that pass the supplement keyword filter and vendor blocklist, then reports any that lack enough data (mg, count, grams) for the analyzer to compute `activeGrams`. For each gap, prints the product handle, what data was extracted, what is missing, and a suggested `vendor_rules.json` override snippet. Use this after scraping to discover new products that need manual overrides.

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
cmd/main.go                  CLI entry point. Flags: --refresh, --supplements, --audit. After saving analysis_report.json, extracts all NeedsReview entries into data/needs_review.json.
internal/
  config/vendors.go          Vendor registry (name, URL, scraper type, cloudflare flag).
  models/types.go            Core structs: Vendor, Product, Variant, Analysis (with JSON tags, including ActiveGrams, GrossGrams, Multiplier, MultiplierLabel, IsSubscription, NeedsReview, and ReviewReason).
  parser/analyzer.go         Hybrid Catalog/Regex Engine with three-tier mass resolution and active/gross mass disambiguation. Checks variantBlocklist to skip ghost variants. ActiveGrams priority: variantOverrides[v.Title] > forceActiveGrams > regex pipeline. GrossGrams priority: variantGrossOverrides[v.Title] > reLabelGrams/reLabelKg regex on product/variant titles. Pure Powder fallback: if no dirty keywords and regex-resolved, activeGrams = grossGrams. Pack multiplier always runs. CostPerGram and EffectiveCost use activeGrams as denominator. Triage Engine scans regex-resolved products against dirtyKeywords and sets NeedsReview/ReviewReason. Returns []models.Analysis (all valid variants). Strips vendor name from product title. Populates multiplier and multiplier label. Emits synthetic "Subscribe & Save" entries when vendor has globalSubscriptionDiscount > 0.
  parser/audit.go            Gap detector. Finds products that pass filters but lack data for analysis (activeGrams == 0). Prints override suggestions using forceActiveGrams/forceServingMg format.
  rules/rules.go             Loads vendor_rules.json. ApplyRules() evaluates product-level blocklist only (returns true/false). No data enrichment — overrides and variantBlocklist are consumed directly by the analyzer.
  scraper/router.go          Routes vendors to the correct scraper engine.
  scraper/shopify.go         Shopify products.json scraper with pagination safety.
  scraper/magento.go         Magento swatch-renderer JSON + bulk pricing scraper.
  scraper/ld+json.go         Schema.org LD+JSON @graph scraper.
  storage/json_store.go      Reads/writes data/*.json files. SaveReport() writes analysis_report.json.
data/
  analysis_report.json       ★ THE INTEGRATION POINT. Pre-computed Analysis array. Frontend reads ONLY this.
  needs_review.json          Triage Engine output. Subset of analysis_report.json entries where needs_review == true. Written by cmd/main.go after every run. Operator reviews this to decide which products need overrides in vendor_rules.json.
  vendor_rules.json          Blocklists and manual dosage overrides per vendor.
  *.json                     Scraped raw product data (one file per vendor). NOT read by the frontend.
web/
  app/layout.tsx             Root layout. Dark theme, font loading, metadata.
  app/page.tsx               Main SSG page. Reads analysis_report.json, renders table. Zero parsing logic.
  app/globals.css            Tailwind v4 styles, rank badges, type badges, custom scrollbar.
  components/
    ProductTable.tsx          Desktop table + mobile card layout. Sorting, filtering.
    SupplementFilter.tsx      Pill-style filter tabs (All, NMN, NAD+, TMG, Resveratrol, Creatine).
    TypeBadge.tsx             Colored badge for product type (Capsules, Powder, Tablets, Gel, etc.).
    RankBadge.tsx             Gold/silver/bronze for top 3, plain number for the rest.
  lib/
    data.ts                  Reads data/analysis_report.json. Maps snake_case → camelCase. Single file.
    types.ts                 Analysis interface (camelCase). The only data type the frontend uses.
    vendors.ts               Vendor registry with base URLs.
  next.config.ts             Static export, remote image patterns for vendor CDNs.
  package.json               Next.js 15, React 19, Tailwind CSS v4.
  tsconfig.json              Path aliases (@/*).
.github/workflows/scrape.yml Daily cron job: scrape → diff → commit → deploy.
```

## Vendor Rules (`data/vendor_rules.json`)

Each vendor can have:

- **`blocklist`**: Product title substrings to reject at the product level (e.g. `"Bundle"`, `"Subscription"`). Evaluated by `ApplyRules()` before the product reaches the analyzer.
- **`variantBlocklist`**: Variant title substrings to reject at the variant level (e.g. `"30 SERV"`, `"Sample"`). Evaluated inside the analyzer's variant loop — matched variants are skipped via `continue`. Use this to suppress ghost variants that share a product handle with valid variants.
- **`overrides`**: Keyed by product handle. Each override is a `ProductSpec` with immutable math fields:
  - `forceType` (string): Product type override (e.g. `"Capsules"`, `"Powder"`, `"Tablets"`, `"Gel"`). Bypasses string-matching type classification.
  - `forceActiveGrams` (float): Pre-computed total active ingredient mass in grams. Mapped to `ActiveGrams` in the Analysis output. When > 0, the regex mass-extraction pipeline is bypassed entirely. Formula: `mg_per_serving × count / 1000`. This is the denominator for all cost calculations.
  - `forceServingMg` (float): Per-serving mg. Informational/documentation field — not consumed by the analyzer, but aids operators in verifying the `forceActiveGrams` calculation.
  - `variantOverrides` (map[string]float64): Per-variant active ingredient grams, keyed by exact variant title string. When a variant title matches a key and the value is > 0, it takes highest priority — bypassing both `forceActiveGrams` and the regex pipeline. Use this when a single product handle groups variants with drastically different active weights (e.g. Nutricost "500 GMS" vs "30 SERV" under one handle).
  - `variantGrossOverrides` (map[string]float64): Per-variant gross (label) weight in grams, keyed by exact variant title string. When a variant title matches a key and the value is > 0, the regex label-weight extraction is bypassed for that variant. Use this for variants whose titles lack standard gram/kg patterns (e.g., `"30 SERV"`) where the physical container weight is known but not parseable.
- **`globalSubscriptionDiscount`**: A float between 0 and 1 representing the fractional discount for subscription purchases (e.g., `0.10` = 10% off). When set, the analyzer emits a second "Subscribe & Save" entry for every valid variant of that vendor's products, with `is_subscription: true` and the discounted price. Used for vendors whose Shopify APIs do not expose subscription pricing directly.

Example:

```json
{
  "Nutricost": {
    "blocklist": ["5-HTP", "Carnitine"],
    "globalSubscriptionDiscount": 0.20,
    "variantBlocklist": ["Unflavored / 30 SERV"],
    "overrides": {
      "nutricost-creatine-monohydrate-powder-500-grams": {
        "forceType": "Powder",
        "variantOverrides": {
          "Frozen Lemonade / 30 SERV": 150.0,
          "Fruit Punch / 500 GMS": 380.0
        },
        "variantGrossOverrides": {
          "Frozen Lemonade / 30 SERV": 207.0
        }
      }
    }
  },
  "Renue By Science": {
    "blocklist": ["Cream", "Serum", "Pet"],
    "globalSubscriptionDiscount": 0.10,
    "overrides": {
      "lipo-nmn-powdered-liposomal-nmn2": {
        "forceType": "Capsules",
        "forceActiveGrams": 22.5,
        "forceServingMg": 250,
        "variantOverrides": {
          "60 Capsules": 15.0,
          "90 Capsules": 22.5
        }
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

## Data Pipeline

```
Go Scraper → data/*.json (raw) → analyzer.go → data/analysis_report.json → Next.js (dumb renderer)
```

The Go backend is the single source of truth for all parsing, regex extraction, bioavailability math, multiplier assignment, vendor-name stripping, type classification, and synthetic subscription generation. The analyzer implements a **Hybrid Catalog/Regex Engine** with active/gross mass disambiguation: `activeGrams` (active ingredient mass) is populated via the priority chain (variant override > `forceActiveGrams` > regex pipeline) and serves as the denominator for `CostPerGram` and `EffectiveCost`. `grossGrams` (label weight) is resolved via a two-tier chain: `variantGrossOverrides` (manual per-variant override for titles lacking gram/kg patterns) > `reLabelGrams`/`reLabelKg` regex on product/variant titles — it defaults to 0 for capsule products or when neither override nor regex yields a value. For "Pure Powder" products (no dirty keywords), if `grossGrams` was found and `activeGrams` was regex-resolved (not override), `activeGrams` is set equal to `grossGrams`. Products with `forceActiveGrams` overrides in `vendor_rules.json` bypass the regex mass-extraction pipeline entirely; all other products use the standard regex pipeline. The `rePack` (pack multiplier) regex always runs regardless of override source. `ApplyRules()` performs blocklist filtering only — it does not inject strings into product context. A **Triage Engine** runs after mass extraction: regex-resolved products are scanned against `dirtyKeywords` (flavors, blends, gummies, combos) and flagged with `needs_review: true` / `review_reason`. `cmd/main.go` extracts all flagged entries into `data/needs_review.json` for operator review. The frontend contains zero duplicated logic. It reads `data/analysis_report.json` and renders it. Each product may appear twice in the report — once as a one-time purchase (`is_subscription: false`) and once as a subscription (`is_subscription: true`) — enabling the frontend to toggle between purchase types.

## Frontend

**Stack:** Next.js 15 (App Router), React 19, Tailwind CSS v4, static export (`output: "export"`).

**How it works:**

1. `lib/data.ts` reads `data/analysis_report.json` using `fs.readFileSync` during the build step. It maps the Go backend's snake_case JSON fields (including `active_grams`, `gross_grams`, `is_subscription`) to camelCase TypeScript (including `activeGrams`, `grossGrams`, `isSubscription`) via a `mapEntry()` function.
2. `app/page.tsx` calls `loadReport()`, attaches `VendorInfo` metadata, and passes the result to `ProductTable`. No parsing. No math.
3. `ProductTable` is a client component that provides supplement filtering (pill tabs) and column sorting. Desktop shows a data table with separate Active and Gross columns; mobile (<768px) shows a card layout with Gross shown as subtext below Active when they differ. The Gross column displays "—" when `grossGrams` is 0 or equals `activeGrams`. The True Cost column header has a hover tooltip `(i)` explaining: "Base Price ÷ Bioavailability Multiplier". When a product's `multiplier > 1`, a muted subtext below the True Cost value shows the multiplier and its label (e.g., `(1.5x Lipo Bonus)`).
4. The build produces a fully static `out/` directory. No server, no client-side API calls.

**Allowed frontend math:** Only user-driven state calculations (e.g., a future "Monthly Cost" column based on dosage input). All product-level computation is pre-computed by Go.

