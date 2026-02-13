# Longevity Ranker

A ruthless, high-performance aggregator that answers one question: **"Who has the cheapest authentic NMN (and other longevity supplements) today?"**

Architecture: **Git-Scraper**. Go scrapes vendor sites and writes JSON. Next.js reads JSON at build time. $0/month infrastructure cost.

## Features

- **Multi-vendor price comparison** across Shopify, Magento, and LD+JSON storefronts.
- **Bioavailability-adjusted pricing** (True Cost) — liposomal, sublingual, and gel formulations receive a multiplier that lowers their effective $/gram.
- **Multi-supplement tracking** — NMN, NAD+, TMG, Resveratrol, and Creatine out of the box. Configurable via `--supplements` flag.
- **Currency normalization** — vendor prices in GBP, EUR, CAD, or AUD are converted to USD using static exchange rates (`internal/parser/currency.go`).
- **Cloudflare-safe** — vendors behind Cloudflare (Jinfiniti, Wonderfeel) are flagged with `Cloudflare: true` in the vendor config. The scraper skips them on `--refresh` and uses manually-maintained JSON instead.
- **Override system** — missing dosage data (capsule count, mg per cap) is fixed via hardcoded rules in `data/vendor_rules.json`. No OCR. No image parsing.
- **Pagination safety** — Shopify scraper uses proper URL construction, product deduplication, and a hard page limit (50) to prevent infinite loops.
- **Daily CI/CD** — GitHub Actions workflow scrapes daily, commits changed JSON, and triggers a Vercel build.

## Setup

```
go mod tidy
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

### Filter by supplement type

```
go run cmd/main.go --supplements=nmn,nad
go run cmd/main.go --supplements=creatine
go run cmd/main.go --supplements=tmg,resveratrol
```

Default value: `nmn,nad,tmg,trimethylglycine,resveratrol,creatine`

## Project Structure

```
cmd/main.go                  CLI entry point. Flags: --refresh, --supplements.
internal/
  config/vendors.go          Vendor registry (name, URL, scraper type, currency, cloudflare flag).
  models/types.go            Core structs: Vendor, Product, Variant, Analysis.
  parser/analyzer.go         Math engine. Extracts mg, count, grams from text. Calculates $/gram and True Cost.
  parser/currency.go         Static exchange rate table. Converts to USD.
  rules/rules.go             Loads vendor_rules.json. Applies blocklists and manual overrides.
  scraper/router.go          Routes vendors to the correct scraper engine.
  scraper/shopify.go         Shopify products.json scraper with pagination safety.
  scraper/magento.go         Magento swatch-renderer JSON + bulk pricing scraper.
  scraper/ld+json.go         Schema.org LD+JSON @graph scraper.
  storage/json_store.go      Reads/writes data/*.json files.
data/
  vendor_rules.json          Blocklists and manual dosage overrides per vendor.
  *.json                     Scraped product data (one file per vendor).
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

## Currency

Exchange rates are hardcoded in `internal/parser/currency.go`. Update them periodically via commit. Supported currencies: USD, GBP, EUR, CAD, AUD.

NMN Bio (`.co.uk`) is configured as `Currency: "GBP"`. All other vendors default to `"USD"`.

## Cloudflare-Protected Vendors

Jinfiniti and Wonderfeel are behind Cloudflare. Their `Cloudflare: true` flag causes the scraper to skip live fetching and load from `data/<vendor>.json` instead. To update their data:

1. Manually visit the vendor site.
2. Extract product info into the matching JSON file in `data/`.
3. Commit and push.

## Affiliate Link Generation

Product `Handle` (URL slug or full URL) and the vendor's base URL are available in the data. The frontend constructs affiliate links dynamically:

```
href={`${vendor.URL}${product.Handle}?ref=${AFFILIATE_ID}`}
```

`AFFILIATE_ID` is stored as an environment variable or config value, allowing weekly token rotation without code changes.