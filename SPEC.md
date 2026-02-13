# Longevity Ranker

## 1. Project Overview

**Objective:** Build a ruthless, high-performance, single-page aggregator that answers one question: *"Who has the cheapest authentic NMN (and other longevity supplements) today?"*
**Business Model:** Affiliate marketing via geo-arbitrage and ROI calculation.
**Architecture:** "Git-Scraper" model. $0/month infrastructure cost.

## 2. System Architecture

The system is decoupled into two primary components communicating via static JSON files committed to the repository.

1. **Backend (Go):** Scrapes vendor sites, standardizes data, applies hardcoded business rules/overrides, calculates ROI, and outputs JSON.
2. **CI/CD (GitHub Actions):** Runs the Go script daily. If data changes, it commits the changes and triggers a frontend build.
3. **Frontend (Next.js):** Reads the JSON at build time (SSG), rendering a static, ultra-fast HTML page hosted on Vercel/Cloudflare Pages.

---

## 3. Backend Specification (Go Scraper)

### 3.1. Current State & Workflow

* **Command:** `go run cmd/main.go -refresh` (Scrapes web -> saves to `data/*.json` -> Analyzes -> Outputs table).
* **Command:** `go run cmd/main.go` (Reads local `data/*.json` -> Analyzes -> Outputs table). Instant execution for logic debugging.
* **Scraper Engines (`internal/scraper/`):** * `shopify.go`: Parses `products.json` endpoints.
* `magento.go`: Parses embedded `Magento_Swatches/js/swatch-renderer` JSON configs and extracts HTML metadata.
* `ldjson.go`: Parses Schema.org `@graph` LD+JSON objects.


* **Normalization Layer (`internal/rules/`):** Reads `data/vendor_rules.json`. Applies blocklists (e.g., ignoring "5-HTP") and manual overrides (e.g., forcing capsule counts when missing from HTML).
* **Math Engine (`internal/parser/analyzer.go`):** Calculates `CostPerGram` and applies the Bioavailability Multiplier to calculate `EffectiveCost`.

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
	Vendor        string
	Name          string
	Price         float64
	TotalGrams    float64
	CostPerGram   float64
	EffectiveCost float64
	Type          string 
	ImageURL      string
}

```

---

## 4. Frontend Specification (Next.js)

### 4.1. Tech Stack

* **Framework:** Next.js (App Router).
* **Styling:** Tailwind CSS.
* **Deployment:** Vercel (or Cloudflare Pages).
* **Rendering:** Strictly Static Site Generation (SSG). No client-side fetching to original APIs. No databases.

### 4.2. Data Fetching (SSG)

* Next.js Server Components must read the localized `data/*.json` files directly from the filesystem during the build step using `fs.readFileSync`.
* The build step must aggregate all vendor JSONs, run the parsing/filtering logic (replicating or consuming the Go output), and generate the static HTML.

### 4.3. UI/UX Requirements

* **The Table:** The core UI is a data table sorted by `EffectiveCost` (Lowest to Highest).
* Columns: Rank, Image, Vendor, Product Name, Type (Powder/Capsule/Gel), Base Price, Total Grams, $/Gram, True Cost (Effective Cost).


* **Mobile Optimization:** 60% of traffic is mobile. The table must be horizontally scrollable or pivot into a "Card" layout on `max-width: 768px`.
* **Performance:** Lighthouse score must be >95. Time to Interactive (TTI) < 1.0s.

### 4.4. Frontend Features & Logic

* **Affiliate Routing:** Do not hardcode affiliate links in the JSON. Build a dynamic URL constructor on the frontend:
`href={${vendor.URL}${product.Handle}?ref=${AFFILIATE_ID}}`. This allows mass-updating weekly expiring tokens via environment variables or a single config file.
* **Image Optimization:** Vendor images must be passed through Next.js `<Image src={product.ImageURL} />`. This proxies the image through the Vercel CDN, protecting the site from vendor hotlinking bans and optimizing WebP formats. (Requires configuring `next.config.js` `remotePatterns`).
* **Compliance Banners:** * *EU Warning:* If the user's IP/locale is EU, display: *"NMN is classified as a Novel Food in the EU. Listings are for research/personal import purposes only."* (Can use Vercel Edge Middleware for geo-detection).
* *FDA Footer:* Standard disclaimer required on all pages: *"Not intended to diagnose, treat, cure..."*


* **Trust Signals:** Visually indicate if a product has a verified Certificate of Analysis (CoA). (Boolean passed from backend).

---

## 5. CI/CD Pipeline (GitHub Actions)

Create `.github/workflows/scrape.yml` with the following requirements:

1. **Schedule:** `cron: '0 8 * * *'` (Runs daily).
2. **Environment:** Ubuntu latest, Go 1.21+.
3. **Execution:** Run `go run cmd/main.go -refresh`.
4. **Diff Check:** Check if `data/*.json` files have changed using `git diff`.
5. **Commit & Push:** If changes exist, commit as "Auto-update product data [skip ci]".
6. **Trigger Build:** Trigger the Next.js Vercel build webhook.

---

## 6. Agent Directives & Constraints

* **Rule 1: No Databases.** Do not introduce PostgreSQL, MongoDB, Prisma, or ORMs. The JSON files in the repo are the sole source of truth.
* **Rule 2: Don't Break the Analyzer.** When modifying `analyzer.go`, ensure strict isolation of string parsing. Do not allow HTML tags from `BodyHTML` to leak into `Type` classification.
* **Rule 3: OCR is Banned.** Do not implement image-to-text processing for missing data. Rely entirely on `data/vendor_rules.json` overrides.
* **Rule 4: Brutal Simplicity.** Avoid complex state management (Redux/Zustand) in Next.js. The app is a static table. Keep client-side JavaScript to an absolute minimum.