# Longevity Ranker

## 1. Project Overview

**Objective:** Build a ruthless, high-performance, single-page aggregator that answers one question: *"Who has the cheapest authentic NMN (and other longevity supplements) today?"*
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

* **Command:** `go run cmd/main.go -refresh` (Scrapes web concurrently → saves raw products to `data/*.json` → Analyzes → Saves report to `data/analysis_report.json` → Prints table to stdout).
* **Command:** `go run cmd/main.go` (Reads local `data/*.json` concurrently → Analyzes → Saves report → Prints table). Instant execution for logic debugging.
* **Command:** `go run cmd/main.go -audit` (Runs the normal pipeline, then scans all products that pass the supplement keyword filter and vendor blocklist. Products that lack enough data for the analyzer to compute `activeGrams` are printed with a gap report: what data was extracted, what is missing, and a suggested `vendor_rules.json` override snippet. Combinable with `-refresh`.)
* **Command:** `go run cmd/main.go -pprof` (Starts the pprof HTTP server on `:6060`. Off by default.)
* **Dependency Injection:** There is no global mutable state in the Go backend. `rules.LoadRules()` returns a `rules.Registry` (type alias for `map[string]VendorConfig`). `cmd/main.go` constructs a `parser.Analyzer` struct with the registry and supplement keywords injected as fields, then calls its methods. `rules.ApplyRules()` takes the registry as an explicit parameter.
* **Concurrency Model:** `cmd/main.go` calls `scrapeAll()`, which launches one goroutine per vendor using `sync.WaitGroup`. Each goroutine calls `scrapeOrLoad()` independently and sends its result through a buffered channel. A separate goroutine calls `wg.Wait()` then `close(ch)`. The main goroutine drains the channel sequentially, applies blocklist rules via `rules.ApplyRules(reg, ...)`, and collects products into a `[]vendorProduct` slice. All downstream processing (analysis, sorting, report generation) remains sequential and deterministic.
* **Scraper Engines (`internal/scraper/`):** Scrapers are registered as `FetchFunc` values (type `func(models.Vendor) ([]models.Product, error)`) in a package-level `registry` map keyed by vendor type string. `FetchProducts()` dispatches to the correct function via map lookup — no switch statement. All scrapers share a `DefaultClient` (`*http.Client`) and `NewRequest()`/`FetchBody()` helpers from `client.go`, eliminating duplicate HTTP boilerplate.
  * `shopify.go`: Parses `products.json` endpoints.
  * `magento.go`: Parses embedded `Magento_Swatches/js/swatch-renderer` JSON configs and extracts HTML metadata. All regexps are compiled once at package level.
  * `ld+json.go`: Parses Schema.org `@graph` LD+JSON objects.
* **Normalization Layer (`internal/rules/`):** Reads `data/vendor_rules.json`. `LoadRules()` returns `(Registry, error)` — no global variable. `ApplyRules(reg, vendorName, p)` evaluates only the product-level vendor blocklist and returns `false` to reject a product, `true` to allow it. It performs NO data enrichment or string injection — overrides are consumed directly by the analyzer's Hybrid Engine. The `VendorConfig` struct also carries `VariantBlocklist []string` for skipping ghost variants inside the analyzer loop, and `GlobalSubscriptionDiscount float64` for vendors whose Shopify APIs hide subscription pricing.
* **Regex Extraction Helpers (`internal/parser/extract.go`):** `extractFloat(re, s) (float64, bool)` returns the first captured group as a float64, returning `(0, false)` on no match or non-positive value. `extractFloatFrom(re, sources...)` tries `extractFloat` against each source string in order, implementing the "variant → clean → broad" fallback chains in a single call. `containsAny(s, substrs)` reports whether a string contains any substring from a slice. These three helpers replace ~13 instances of the 3–5 line regex→parse→check pattern across analyzer.go and audit.go.
* **Math Engine (`internal/parser/analyzer.go`):** The `Analyzer` struct holds `Rules rules.Registry` and `Supplements []string`. Its `AnalyzeProduct()` method implements a **Hybrid Catalog/Regex Engine** with three-tier mass resolution and active/gross mass disambiguation. Returns `[]models.Analysis` — one entry per valid variant. Mass extraction is delegated to `Analyzer.extractMass()`, which returns `(capsuleMass, powderMass, usedOverride)`. For **ActiveGrams** extraction (the active ingredient mass), the method evaluates a strict priority chain: **(1)** `spec.VariantOverrides[v.Title]` — per-variant override takes highest priority; **(2)** `spec.ForceActiveGrams` — product-level override bypasses regex; **(3)** standard regex pipeline via `extractFloat`/`extractFloatFrom` helpers. The `rePack` regex (pack multiplier) always runs regardless of override source. `activeGrams = baseMass * packMultiplier`. **GrossGrams** (label weight) is resolved by `Analyzer.extractGrossGrams()` via a two-tier priority chain: **(1)** `spec.VariantGrossOverrides[v.Title]`; **(2)** regex extraction via `reLabelGrams`/`reLabelKg` scanning only label text. Defaults to 0 for capsule-only products. **Pure Powder Fallback:** if the product has no dirty keywords (checked via `containsAny`), GrossGrams was found, and ActiveGrams was regex-resolved, then `activeGrams = grossGrams`. Type classification is delegated to `classifyType()`. Bioavailability multiplier is resolved by `bioavailabilityMultiplier()`. Display name is built by `buildDisplayName()`. Cost metrics are computed by `buildAnalysis()`, which constructs a single `models.Analysis` entry — used for both one-time and subscription entries, eliminating the previous struct-literal duplication. When a vendor has `GlobalSubscriptionDiscount > 0`, a synthetic "Subscribe & Save" entry is emitted via the same `buildAnalysis()` helper. Returns `nil` when the product has no analyzable variants.
* **Triage Engine (`internal/parser/analyzer.go`):** Dirty-data detection is delegated to `Analyzer.triageDirtyData()`. If mass was NOT resolved by an override, the method scans against `dirtyKeywords` using `containsAny` with a special-case guard for `"unflavored"` products. The servings sub-exception flags products with `"serv"` in their identity for manual review. Both one-time and subscription entries inherit the same flag. `cmd/main.go` calls `saveReviewQueue()` to extract flagged entries and write them to `data/needs_review.json`.
* **Audit Gap Detector (`internal/parser/audit.go`):** `Analyzer.AuditProduct()` is a method on the `Analyzer` struct. It runs the same supplement keyword gate (via `Analyzer.matchesSupplement()`) and calls `Analyzer.AnalyzeProduct()` to check if the product is already analyzable. If not, it probes for partial data using `extractFloat`/`extractFloatFrom` helpers and returns an `AuditResult` describing the gap. `FormatAuditReport()` groups results by vendor and renders them as a human-readable stdout report. Triggered by the `-audit` CLI flag.
* **Storage (`internal/storage/json_store.go`):** Uses Go generics: `SaveJSON[T any](path, data)` and `LoadJSON[T any](path)` replace the previous `SaveProducts`, `SaveReport`, and `LoadProducts` functions. `VendorFilename()` converts a vendor name to its JSON file path (e.g., `"Do Not Age"` → `"data/do_not_age.json"`).

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
	ActiveGrams     float64 `json:"active_grams"`
	GrossGrams      float64 `json:"gross_grams"`
	CostPerGram     float64 `json:"cost_per_gram"`
	EffectiveCost   float64 `json:"effective_cost"`
	Multiplier      float64 `json:"multiplier"`
	MultiplierLabel string  `json:"multiplier_label"`
	Type            string  `json:"type"`
	ImageURL        string  `json:"image_url"`
	IsSubscription  bool    `json:"is_subscription"`
	NeedsReview     bool    `json:"needs_review"`
	ReviewReason    string  `json:"review_reason,omitempty"`
}
```

The `Analysis` struct is the schema for `data/analysis_report.json`. JSON field names use snake_case. The frontend maps these to camelCase at the data-loading boundary (`web/lib/data.ts`).

#### Field Notes

* **`Name`**: The analyzer strips the vendor name prefix from the product title before assigning it. Stripping is case-insensitive. Example: vendor `"Nutricost"`, title `"Nutricost Creatine Monohydrate"` → `Name` becomes `"Creatine Monohydrate"`. If stripping would produce an empty string, the original title is kept.
* **`ActiveGrams`**: The total active ingredient mass in grams. This is the denominator for `CostPerGram` and `EffectiveCost` calculations. Populated by the Hybrid Engine's priority chain: variant override (`VariantOverrides`) > product override (`ForceActiveGrams`) > regex pipeline. For "Pure Powder" products (no dirty keywords), if a label weight (GrossGrams) was found and mass was regex-resolved (not override), ActiveGrams is set equal to GrossGrams.
* **`GrossGrams`**: The physical weight printed on the product label (e.g., "500 GMS", "1 KG"). Resolved via a three-tier priority chain: **(1)** `VariantGrossOverrides[v.Title]` — per-variant manual override for variants whose titles lack standard gram/kg patterns (e.g., `"30 SERV"`); **(2)** regex extraction via `reLabelGrams`/`reLabelKg` scanning `variant.Title` and `product.Title` only — never `body_html`; **(3)** **Pure Powder Fallback** — if the product type is `"Powder"`, `grossGrams` is still `0` after overrides and regex, and the product is NOT flagged for review (`!needsReview`), then `grossGrams` is set equal to `activeGrams`. Rationale: an unflagged powder product is 100% pure active ingredient, so the container weight equals the active weight. This covers products with minimalist titles (e.g., Blueprint's `"Creatine"`) where no gram/kg pattern exists for regex to match. Defaults to `0` for capsule-only products, tablets, or flagged powders where neither override, regex, nor fallback applies. NOT used in cost calculations — exists solely for frontend transparency. The frontend and CLI display the value whenever `grossGrams > 0`; when `0`, they display "—".
* **`Multiplier`**: The bioavailability multiplier applied to `CostPerGram` to produce `EffectiveCost` (i.e., `EffectiveCost = CostPerGram / Multiplier`). Defaults to `1.0` for standard formulations. Values: `1.5` for liposomal, `1.1` for sublingual/gel/tablet.
* **`MultiplierLabel`**: Human-readable label for the multiplier reason. Empty string when `Multiplier` is `1.0`. Possible values: `"Lipo Bonus"`, `"Sublingual"`, `"Gel Bonus"`, `"Tablet Bonus"`.
* **`IsSubscription`**: `true` when the entry is a synthetic "Subscribe & Save" row generated by the analyzer. `false` for standard one-time purchase entries. The frontend uses this field to power a purchase-type toggle.
* **`NeedsReview`**: `true` when the Triage Engine detected a dirty keyword in a product whose mass was resolved by regex (no override). `false` when the product has an explicit override or no dirty keyword was found. Flagged entries are also written to `data/needs_review.json` by `cmd/main.go`.
* **`ReviewReason`**: Human-readable reason for the flag. Format: `"Detected dirty keyword: <word>"`. Empty string when `NeedsReview` is `false`.

---

## 4. Frontend Specification (Next.js)

### 4.1. Tech Stack

* **Framework:** Next.js 15 (App Router), React 19.
* **Styling:** Tailwind CSS v4 (PostCSS plugin `@tailwindcss/postcss`).
* **Deployment:** Vercel (or Cloudflare Pages). Static export (`output: "export"` in `next.config.ts`).
* **Rendering:** Strictly Static Site Generation (SSG). No client-side fetching to original APIs. No databases. Build produces `web/out/` — a flat directory of HTML/CSS/JS.

### 4.2. Data Fetching (SSG)

* `web/lib/data.ts` reads `data/analysis_report.json` from the filesystem at build time using `fs.readFileSync`. The data directory is resolved relative to the `web/` working directory (`path.resolve(process.cwd(), '..', 'data')`).
* `data.ts` maps the snake_case JSON fields (`active_grams`, `gross_grams`, `cost_per_gram`, `effective_cost`, `multiplier`, `multiplier_label`, `image_url`, `is_subscription`) to camelCase (`activeGrams`, `grossGrams`, `costPerGram`, `effectiveCost`, `multiplier`, `multiplierLabel`, `imageURL`, `isSubscription`) via a private `RawReportEntry` interface and a `mapEntry()` function. All downstream code uses the camelCase `Analysis` type.
* `web/app/page.tsx` calls `loadReport()` in a Server Component, enriches each entry with `VendorInfo` from `web/lib/vendors.ts`, and passes the result to `ProductTable`.
* **The frontend contains zero parsing logic.** No regexes, no mg/count extraction, no bioavailability multipliers, no type classification. All of that lives exclusively in the Go backend's `analyzer.go`. The frontend is a dumb renderer of pre-computed data.

### 4.3. UI/UX Requirements

* **The Table:** The core UI is a data table sorted by `effectiveCost` (Lowest to Highest). Columns: Rank (gold/silver/bronze badges for top 3), Image, Vendor, Product Name, Type (colored pill badge), Base Price, Active (grams), Gross (grams), $/Gram, True Cost, Buy link. The "Active" column shows `activeGrams` (the denominator for cost math). The "Gross" column shows `grossGrams` whenever it is `> 0` (including when it equals Active — this is the expected state for pure powders); it shows "—" only when `grossGrams` is `0`, which is the correct state for Capsules and Tablets that do not advertise a gross powder weight.
* **True Cost Transparency:** The True Cost column header includes a hover tooltip `(i)` explaining: "Base Price ÷ Bioavailability Multiplier". When a product has a `multiplier > 1`, a muted subtext is rendered below the True Cost value showing the multiplier and its label (e.g., `(1.5x Lipo Bonus)`, `(1.1x Sublingual)`). This subtext appears in both the desktop table rows and the mobile card layout. Products with a `1.0` multiplier show no subtext.
* **Supplement Filter:** Pill-style tabs at the top filter by supplement type: All, NMN, NAD+, TMG, Resveratrol, Creatine. Implemented as a client component (`SupplementFilter.tsx`) with `useState`. Filtering is keyword-based on the product name/handle/vendor string — no re-analysis.
* **Column Sorting:** Clicking Price, $/Gram, or True Cost column headers toggles ascending/descending sort. Active sort column shows a directional arrow indicator.
* **Mobile Layout:** Below `md` breakpoint (768px), the table is hidden and replaced by a card layout. Each card shows rank badge, product image, vendor name, type badge, product name, a 2×2 stats grid (Price, Total, $/Gram, True Cost), and a full-width "View Deal" button.
* **Performance:** Static export. First Load JS is ~105 kB. No client-side API calls. All product data is baked into the HTML at build time.

### 4.4. Frontend Features & Logic

* **Image Handling:** Product images use a standard `<img>` tag with `loading="lazy"`. `next.config.ts` defines `remotePatterns` for all vendor CDN hostnames (cdn.shopify.com, donotage.org, renuebyscience.com, etc.). Images are set to `unoptimized: true` for static export compatibility.
* **Compliance Banners:** All rendered in the page footer (`web/app/page.tsx`):
  * *FDA Disclaimer:* "These statements have not been evaluated by the Food and Drug Administration..."
  * *EU Notice:* "NMN is classified as a Novel Food in the European Union. Listings are provided for research and personal import purposes only."
* **Vendor Registry:** `web/lib/vendors.ts` maps each vendor name to its base URL and whether the handle is a full URL or a slug. It does not reference any raw data files.
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