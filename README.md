# Longevity Ranker

## TODO:
- Add more vendors
- Solve inconsistencies in the data (e.g. Do Not Age / Pure NMN (366 Capsules)/ Capsules / $440.00 / 183.0g / $2.40) - The calculated gram is wrong. It is calculating for 30 capsules only.
- Add more products (creatine, TMG, etc.)

You have enough data to implement the core unique value proposition (The **"Effective Cost" Algorithm**), but the logic is missing from your code.

* **Feature: The "True Cost" (Bioavailability) Algorithm**
* **Requirement:** "Base Price Per Gram ÷ Bioavailability Multiplier = Effective Cost"
* **Current Status:** Your `analyzer.go` calculates the raw `CostPerGram`. It also successfully identifies the `Type` (Powder, Capsules, Gel).
* **Implementation:** You can add the multipliers (Powder = 1.0, Gel/Liposomal = 1.5, etc.) directly into `analyzer.go` or `main.go`.
* **Why:** This allows you to rank "expensive" Liposomal Gels fairly against cheap Powders, stopping the Gels from looking like a bad deal.


* **Feature: Affiliate Link Generation**
* **Requirement:** "GeniusLink or simple JS redirection logic."
* **Current Status:** You have the product `Handle` (URL slug) and the vendor's base URL.
* **Implementation:** You can add an `AffiliateID` field to your `VendorConfig` in `rules.go`. In `main.go`, you can concatenate `Vendor URL` + `Handle` + `?ref=AffiliateID` to generate the final "Buy Now" link for the frontend.


* **Feature: "Smart" Filtering**
* **Requirement:** "Filter out non-NMN products."
* **Current Status:** Your `rules.go` Blocklist is doing this well. You can refine this further by ensuring "Pre-Orders" or "Out of Stock" items are flagged if that data is available (Shopify usually provides `available: true/false` in the raw JSON, though your current `models.Product` struct doesn't capture it).



#### 2. Critical Missing Data (Must Get Before Launch)

To meet the "MVP Strategy" defined in `requirements.md`, you are missing three critical pieces of data.

##### A. Product Images (Visual Trust)

* **Requirement:** The table must look professional. A text-only spreadsheet looks suspicious to casual users.
* **Missing:** Your `models.Product` struct in `types.go` does **not** have an `ImageURL` field, and your scrapers are ignoring images.
* **Fix:**
* Update `types.go`: Add `ImageURL string` to the `Product` struct.
* Update `shopify.go`: Extract `images[0].src` from the JSON.
* Update `magento.go`: Regex extract the `<meta property="og:image">` tag.



##### B. Currency Normalization (The "NMN Bio" Problem)

* **Requirement:** "Automatically convert all prices to a single display currency (USD)."
* **Missing:** NMN Bio is a UK company (£ GBP). Do Not Age is US/UK (often USD, but can vary). Your current code treats `45.00` as just a number.
* If NMN Bio sells for **£40**, your ranker sees **$40**.
* In reality, £40 is **~$50 USD**. This gives NMN Bio an unfair advantage in the ranking.


* **Fix:**
* Add a `Currency` field to the `Vendor` struct in `vendors.go` (e.g., "USD", "GBP").
* In `analyzer.go` or `main.go`, multiply non-USD prices by a static exchange rate (sufficient for MVP) before ranking.



##### C. Availability / Stock Status

* **Requirement:** Don't send traffic to out-of-stock products (bad user experience).
* **Missing:** The Shopify scraper sees `available: true` in the raw JSON but discards it.
* **Fix:** Add `IsAvailable bool` to your `models.Product` and filter out `false` items in the `main.go` loop.

#### Summary Roadmap

| Feature | Can Build Now? | Action Required |
| --- | --- | --- |
| **ROI Calculator** | ✅ Yes | Update `analyzer.go` to use `Type` for Bioavailability math. |
| **Affiliate Links** | ✅ Yes | Add IDs to `rules.go` and generate links in `main.go`. |
| **Product Images** | ❌ No | Update `types.go` and all scrapers to fetch `ImageURL`. |
| **Currency Fix** | ❌ No | Update `vendors.go` with currency codes; add conversion math. |
| **Stock Status** | ❌ No | Update `shopify.go` to capture `available` field. |

## Setup

```go
go mod tidy
```

## Usage

Scrape data and rank products
```go
go run cmd/main.go -refresh
```

Rank products with local data
```go
go run cmd/main.go
```