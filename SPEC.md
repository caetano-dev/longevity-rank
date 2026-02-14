# Longevity Ranker

## 1. Project Overview

**Objective:** Build a ruthless, high-performance, single-page aggregator that answers one question: *"Who has the cheapest authentic NMN (and other longevity supplements) today?"*
**Business Model:** Affiliate marketing via geo-arbitrage and ROI calculation.
**Architecture:** "Git-Scraper" model. $0/month infrastructure cost.

## 2. System Architecture

The system is decoupled into two primary components communicating via a single static JSON file committed to the repository.

1. **Backend (Go):** Scrapes vendor sites, standardizes data, applies hardcoded business rules/overrides, calculates ROI, and outputs `data/analysis_report.json`.
2. **CI/CD (GitHub Actions):** Runs the Go script daily. If data changes, it commits the changes and triggers a frontend build.
3. **Frontend (Next.js):** Reads `data/analysis_report.json` at build time (SSG), rendering a static, ultra-fast HTML page hosted on Vercel/Cloudflare Pages.

### 2.1. Integration Point

**`data/analysis_report.json`** is the sole contract between the Go backend and the Next.js frontend. The Go backend writes it. The frontend reads it. No other data files cross the boundary.

* The Go backend scrapes raw product data into `data/*.json` (one file per vendor) for its own internal use. These raw files are **not** consumed by the frontend.
* The backend applies vendor rules, runs the math engine, and serializes the final `[]models.Analysis` array to `data/analysis_report.json` via `storage.SaveReport()`.
* The frontend reads only `data/analysis_report.json` via `lib/data.ts`. It performs zero parsing, zero regex extraction, zero bioavailability math. It is a dumb renderer.

---

## 3. Backend Specification (Go Scraper)

### 3.1. Current State & Workflow

* **Command:** `go run cmd/main.go -refresh` (Scrapes web → saves raw products to `data/*.json` → Analyzes → Saves report to `data/analysis_report.json` → Prints table to stdout).
* **Command:** `go run cmd/main.go` (Reads local `data/*.json` → Analyzes → Saves report → Prints table). Instant execution for logic debugging.
* **Command:** `go run cmd/main.go -audit` (Runs the normal pipeline, then scans all products that pass the supplement keyword filter and vendor blocklist. Products that lack enough data for the analyzer to compute `totalGrams` are printed with a gap report: what data was extracted, what is missing, and a suggested `vendor_rules.json` override snippet. Combinable with `-refresh`.)
* **Scraper Engines (`internal/scraper/`):**
  * `shopify.go`: Parses `products.json` endpoints.
  * `magento.go`: Parses embedded `Magento_Swatches/js/swatch-renderer` JSON configs and extracts HTML metadata.
  * `ldjson.go`: Parses Schema.org `@graph` LD+JSON objects.
* **Normalization Layer (`internal/rules/`):** Reads `data/vendor_rules.json`. Applies blocklists (e.g., ignoring "5-HTP") and manual overrides (e.g., forcing capsule counts when missing from HTML). The `VendorConfig` struct also carries `GlobalSubscriptionDiscount float64` for vendors whose Shopify APIs hide subscription pricing — the analyzer uses this value to synthesize discounted subscription entries.
* **Math Engine (`internal/parser/analyzer.go`):** `AnalyzeProduct()` returns `[]models.Analysis` — a slice containing one entry per valid variant. When a vendor has a `GlobalSubscriptionDiscount > 0` in `vendor_rules.json`, a second synthetic "Subscribe & Save" entry is emitted for each variant with the discounted price, `" (Subscribe & Save)"` appended to its `Name`, and `IsSubscription: true`. The function evaluates ALL valid variants (no min-cost gatekeeper). Returns `nil` when the product has no analyzable variants. `cmd/main.go` flattens all returned slices into the master report via `append(report, analyses...)`.
* **Audit Gap Detector (`internal/parser/audit.go`):** `AuditProduct()` runs the same extraction pipeline as `AnalyzeProduct()` but never silently discards products. For each product that passes the supplement keyword filter and vendor blocklist but yields `totalGrams == 0`, it returns an `AuditResult` struct listing the product handle, what data was extracted (mg, count, grams), what is missing, and a suggested `vendor_rules.json` override snippet. `FormatAuditReport()` groups results by vendor and renders them as a human-readable stdout report. Triggered by the `-audit` CLI flag.
* **Report Output (`internal/storage/json_store.go`):** `SaveReport()` serializes the sorted `[]models.Analysis` to `data/analysis_report.json`. Called by `cmd/main.go` after sorting, immediately before printing to stdout.

### 3.2. Data Models (`internal/models/types.go`)

Agents must adhere to these structs when modifying scrapers:

```go
type Product struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	Context  string    `json:"context"`
	Handle   string    `json:"handle"`
	BodyHTML string    `json:"body_html"`
	ImageURL string    `json:"image_url"`
	Variants []Variant `json:"variants"`
}

type Variant struct {
	Price     string `json:"price"`
	Title     string `json:"title"`
	Available bool   `json:"available"`
}

type Analysis struct {
	Vendor          string  `json:"vendor"`
	Name            string  `json:"name"`
	Handle          string  `json:"handle"`
	Price           float64 `json:"price"`
	TotalGrams      float64 `json:"total_grams"`
	CostPerGram     float64 `json:"cost_per_gram"`
	EffectiveCost   float64 `json:"effective_cost"`
	Multiplier      float64 `json:"multiplier"`
	MultiplierLabel string  `json:"multiplier_label"`
	Type            string  `json:"type"`
	ImageURL        string  `json:"image_url"`
	IsSubscription  bool    `json:"is_subscription"`
}
```

The `Analysis` struct is the schema for `data/analysis_report.json`. JSON field names use snake_case. The frontend maps these to camelCase at the data-loading boundary (`web/lib/data.ts`).

#### Field Notes

* **`Name`**: The analyzer strips the vendor name prefix from the product title before assigning it. Stripping is case-insensitive. Example: vendor `"Nutricost"`, title `"Nutricost Creatine Monohydrate"` → `Name` becomes `"Creatine Monohydrate"`. If stripping would produce an empty string, the original title is kept.
* **`Multiplier`**: The bioavailability multiplier applied to `CostPerGram` to produce `EffectiveCost` (i.e., `EffectiveCost = CostPerGram / Multiplier`). Defaults to `1.0` for standard formulations. Values: `1.5` for liposomal, `1.1` for sublingual/gel/tablet.
* **`MultiplierLabel`**: Human-readable label for the multiplier reason. Empty string when `Multiplier` is `1.0`. Possible values: `"Lipo Bonus"`, `"Sublingual"`, `"Gel Bonus"`, `"Tablet Bonus"`.
* **`IsSubscription`**: `true` when the entry is a synthetic "Subscribe & Save" row generated by the analyzer. `false` for standard one-time purchase entries. The frontend uses this field to power a purchase-type toggle.

---

## 4. Frontend Specification (Next.js)

### 4.1. Tech Stack

* **Framework:** Next.js 15 (App Router), React 19.
* **Styling:** Tailwind CSS v4 (PostCSS plugin `@tailwindcss/postcss`).
* **Deployment:** Vercel (or Cloudflare Pages). Static export (`output: "export"` in `next.config.ts`).
* **Rendering:** Strictly Static Site Generation (SSG). No client-side fetching to original APIs. No databases. Build produces `web/out/` — a flat directory of HTML/CSS/JS.

### 4.2. Data Fetching (SSG)

* `web/lib/data.ts` reads `data/analysis_report.json` from the filesystem at build time using `fs.readFileSync`. The data directory is resolved relative to the `web/` working directory (`path.resolve(process.cwd(), '..', 'data')`).
* `data.ts` maps the snake_case JSON fields (`total_grams`, `cost_per_gram`, `effective_cost`, `multiplier`, `multiplier_label`, `image_url`, `is_subscription`) to camelCase (`totalGrams`, `costPerGram`, `effectiveCost`, `multiplier`, `multiplierLabel`, `imageURL`, `isSubscription`) via a private `RawReportEntry` interface and a `mapEntry()` function. All downstream code uses the camelCase `Analysis` type.
* `web/app/page.tsx` calls `loadReport()` in a Server Component, enriches each entry with `VendorInfo` from `web/lib/vendors.ts` (for affiliate link construction), and passes the result to `ProductTable`.
* **The frontend contains zero parsing logic.** No regexes, no mg/count extraction, no bioavailability multipliers, no type classification. All of that lives exclusively in the Go backend's `analyzer.go`. The frontend is a dumb renderer of pre-computed data.

### 4.3. UI/UX Requirements

* **The Table:** The core UI is a data table sorted by `effectiveCost` (Lowest to Highest). Columns: Rank (gold/silver/bronze badges for top 3), Image, Vendor, Product Name, Type (colored pill badge), Base Price, Total Grams, $/Gram, True Cost, Buy link.
* **True Cost Transparency:** The True Cost column header includes a hover tooltip `(i)` explaining: "Base Price ÷ Bioavailability Multiplier". When a product has a `multiplier > 1`, a muted subtext is rendered below the True Cost value showing the multiplier and its label (e.g., `(1.5x Lipo Bonus)`, `(1.1x Sublingual)`). This subtext appears in both the desktop table rows and the mobile card layout. Products with a `1.0` multiplier show no subtext.
* **Supplement Filter:** Pill-style tabs at the top filter by supplement type: All, NMN, NAD+, TMG, Resveratrol, Creatine. Implemented as a client component (`SupplementFilter.tsx`) with `useState`. Filtering is keyword-based on the product name/handle/vendor string — no re-analysis.
* **Column Sorting:** Clicking Price, $/Gram, or True Cost column headers toggles ascending/descending sort. Active sort column shows a directional arrow indicator.
* **Mobile Layout:** Below `md` breakpoint (768px), the table is hidden and replaced by a card layout. Each card shows rank badge, product image, vendor name, type badge, product name, a 2×2 stats grid (Price, Total, $/Gram, True Cost), and a full-width "View Deal" button.
* **Performance:** Static export. First Load JS is ~105 kB. No client-side API calls. All product data is baked into the HTML at build time.

### 4.4. Frontend Features & Logic

* **Affiliate Routing:** `web/lib/vendors.ts` exports `buildAffiliateUrl(vendor, handle, affiliateId)`. Shopify vendors construct `{baseUrl}/products/{handle}?ref={AFFILIATE_ID}`. Full-URL vendors (Do Not Age, Jinfiniti, Wonderfeel) use the handle as-is with `?ref=` appended. `AFFILIATE_ID` is read from `process.env.AFFILIATE_ID` at build time. When empty, links point to bare product URLs.
* **Image Handling:** Product images use a standard `<img>` tag with `loading="lazy"`. `next.config.ts` defines `remotePatterns` for all vendor CDN hostnames (cdn.shopify.com, donotage.org, renuebyscience.com, etc.). Images are set to `unoptimized: true` for static export compatibility.
* **Compliance Banners:** All rendered in the page footer (`web/app/page.tsx`):
  * *FDA Disclaimer:* "These statements have not been evaluated by the Food and Drug Administration..."
  * *EU Notice:* "NMN is classified as a Novel Food in the European Union. Listings are provided for research and personal import purposes only."
  * *Affiliate Disclosure:* "This site may earn a commission from qualifying purchases..."
* **Vendor Registry:** `web/lib/vendors.ts` maps each vendor name to its base URL and whether the handle is a full URL or a slug. This is used solely for constructing affiliate links. It does not reference any raw data files.
* **Allowed Frontend Math:** The only calculations permitted on the frontend are user-driven state computations (e.g., a future "Monthly Cost" column based on user dosage input). All product-level math ($/gram, effective cost, multiplier, type classification) is computed by the Go backend and consumed as-is.

---

## 5. CI/CD Pipeline (GitHub Actions)

`.github/workflows/scrape.yml` requirements:

1. **Schedule:** `cron: '0 8 * * *'` (Runs daily).
2. **Environment:** Ubuntu latest, Go 1.21+.
3. **Execution:** Run `go run cmd/main.go -refresh`.
4. **Diff Check:** Check if `data/*.json` files have changed using `git diff`. This includes both raw vendor files and `analysis_report.json`.
5. **Commit & Push:** If changes exist, commit as "Auto-update product data [skip ci]".
6. **Trigger Build:** Trigger the Next.js Vercel build webhook.

---

## 6. Agent Directives & Constraints

* **Rule 1: No Databases.** Do not introduce PostgreSQL, MongoDB, Prisma, or ORMs. The JSON files in the repo are the sole source of truth.
* **Rule 2: Don't Break the Analyzer.** When modifying `analyzer.go`, ensure strict isolation of string parsing. Do not allow HTML tags from `BodyHTML` to leak into `Type` classification.
* **Rule 3: OCR is Banned.** Do not implement image-to-text processing for missing data. Rely entirely on `data/vendor_rules.json` overrides.
* **Rule 4: Brutal Simplicity.** Avoid complex state management (Redux/Zustand) in Next.js. The app is a static table. Keep client-side JavaScript to an absolute minimum.
* **Rule 5: No Duplicated Logic.** All parsing, regex extraction, bioavailability math, and type classification live exclusively in the Go backend (`analyzer.go`). The frontend reads `data/analysis_report.json` and renders it. Do not re-implement or duplicate the analyzer in TypeScript or any other language.
* **Rule 6: Single Integration Point.** `data/analysis_report.json` is the sole contract between the backend and frontend. If the `Analysis` struct changes in Go, update the `RawReportEntry` interface in `web/lib/data.ts` and the `Analysis` interface in `web/lib/types.ts` to match. Keep `SPEC.md` in sync per `AGENTS_DOCS_PROTOCOL.md`.